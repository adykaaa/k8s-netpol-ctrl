package event

import (
	"errors"
	"fmt"
	"log"

	attr "github.com/adykaaa/k8s-netpol-ctrl/handlers/attribute"
	np "github.com/adykaaa/k8s-netpol-ctrl/handlers/networkpolicy"
	"github.com/adykaaa/k8s-netpol-ctrl/handlers/object"
	networkingv1 "k8s.io/api/networking/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
)

type NetworkPolicyHandler interface {
	NewPolicy(name string, namespace string, podSelectorLabels map[string]string, targetPodLabels map[string][]string) (*networkingv1.NetworkPolicy, error)
	GetPolicyByPodLabels(namespace string, podLabels map[string]string) (*networkingv1.NetworkPolicy, error)
	AppendLabelsToPeers(targetPodLabels map[string][]string) (ingressPeers []networkingv1.NetworkPolicyPeer, egressPeers []networkingv1.NetworkPolicyPeer, err error)
}

type ObjectHandler interface {
	AddLabel() error
	Mutate(action object.Action) error
}

type AttributeHandler interface {
	ConvertLabels(targetPodLabels map[string]string) map[string][]string
	GetLocalEnvVars(obj metav1.Object) (map[string]string, error)
	GetLabelsFromEnvVars(envVars map[string]string) (map[string][]string, error)
	MergeLabels(targetLabels ...map[string][]string) (map[string][]string, error)
}

type Handler struct {
	Client               kubernetes.Interface
	DyanmicClient        dynamic.Interface
	NetworkPolicyHandler NetworkPolicyHandler
	ObjectHandler        ObjectHandler
	AttributeHandler     AttributeHandler
}

/*
handleLabelChange modifies the existing NetworkPolicy if the object's labels have changed. It clears all existing LabelSelectorRequirements
which were targetting pods based on the old object's labels, and sets them up according to the new object's labels
*/
func (h *Handler) handleLabelChange(oldLabels map[string]string, newLabels map[string]string, p *networkingv1.NetworkPolicy) error {
	p.Spec.PodSelector.MatchLabels = newLabels
	convNewLabels := h.AttributeHandler.ConvertLabels(newLabels)
	convOldLabels := h.AttributeHandler.ConvertLabels(oldLabels)

	np.RemoveOldLabels(convOldLabels, p.Spec.Ingress, p.Spec.Egress)

	ingressPol, egressPol, err := h.NetworkPolicyHandler.AppendLabelsToPeers(convNewLabels)
	if err != nil {
		return err
	}

	np.ExtendPeers(ingressPol, egressPol, p)
	return nil
}

/*
handleEnvVarChange handles the case when during an Update event, the metaObject's environment variables which are pointing to
cluster.local addresses have changed, and updates the metaObject's NetworkPolicy accordingly by adding the necessary labels to
the policy's peers.
*/
func (h *Handler) handleEnvVarChange(objLabels map[string]string, envVars map[string]string, p *networkingv1.NetworkPolicy) error {
	el, err := h.AttributeHandler.GetLabelsFromEnvVars(envVars)
	if err != nil {
		return err
	}

	podLabels, err := h.AttributeHandler.MergeLabels(h.AttributeHandler.ConvertLabels(objLabels), el)
	if err != nil {
		return err
	}

	ingressPol, egressPol, err := h.NetworkPolicyHandler.AppendLabelsToPeers(podLabels)
	if err != nil {
		return err
	}
	np.ExtendPeers(ingressPol, egressPol, p)

	return nil
}

// HandleAdd handles the case when a K8s object of interest is added to the cluster, and creates a NetworkPolicy for it
func (h *Handler) HandleAdd(obj interface{}) error {
	var p *networkingv1.NetworkPolicy
	objLabels, metaObj, err := object.ConvertToMeta(obj)
	if err != nil {
		return err
	}
	policyName := fmt.Sprintf("%s-%s-netpol", metaObj.GetName(), metaObj.GetNamespace())

	// we don't mess around in the kube-system namespace
	if metaObj.GetNamespace() == "kube-system" {
		return errors.New("objects in the kube-system namespace won't be modified")
	}

	if len(metaObj.GetLabels()) == 0 {
		h.ObjectHandler = object.NewHandler(h.DyanmicClient, metaObj)
		if err := h.ObjectHandler.AddLabel(); err != nil {
			return err
		}
	}

	envVars, err := h.AttributeHandler.GetLocalEnvVars(metaObj)
	if err != nil && !errors.Is(err, attr.ErrNoEnvVars) {
		return err
	}

	envLabels, err := h.AttributeHandler.GetLabelsFromEnvVars(envVars)
	if err != nil {
		if errors.Is(err, attr.ErrNoEnvVars) || errors.Is(err, attr.ErrResourceNotFound) {
			p, err = h.NetworkPolicyHandler.NewPolicy(policyName, metaObj.GetNamespace(), objLabels, h.AttributeHandler.ConvertLabels(metaObj.GetLabels()))
			if err != nil {
				return fmt.Errorf("could not deploy new policy for %s. %w", metaObj.GetName(), err)
			}
		} else {
			return err
		}
	} else {
		allLabels, err := h.AttributeHandler.MergeLabels(h.AttributeHandler.ConvertLabels(objLabels), envLabels)
		if err != nil {
			return err
		}

		p, err = h.NetworkPolicyHandler.NewPolicy(policyName, metaObj.GetNamespace(), objLabels, allLabels)
		if err != nil {
			return fmt.Errorf("could not deploy new policy for %s. %w", metaObj.GetName(), err)
		}
	}

	h.ObjectHandler = object.NewHandler(h.DyanmicClient, p)
	err = h.ObjectHandler.Mutate(object.Create)
	if err != nil {
		return err
	}

	log.Printf("NetworkPolicy %s added for %s \n", policyName, metaObj.GetName())
	return nil
}

/*
HandleUpdate handles the case when a K8s object of interest is updated in the cluster, provided the object's environment variables
or labels have changed
*/
func (h *Handler) HandleUpdate(oldObj, newObj interface{}) error {
	newLabels, newMetaObj, err := object.ConvertToMeta(newObj)
	if err != nil {
		return err
	}

	// we don't mess around in the kube-system namespace
	if newMetaObj.GetNamespace() == "kube-system" {
		return errors.New("objects in the kube-system namespace won't be modified")
	}

	oldLabels, oldMetaObj, err := object.ConvertToMeta(oldObj)
	if err != nil {
		return err
	}

	oldObjEnvVars, err := h.AttributeHandler.GetLocalEnvVars(oldMetaObj)
	if err != nil {
		return err
	}
	newObjEnvVars, err := h.AttributeHandler.GetLocalEnvVars(newMetaObj)
	if err != nil {
		return err
	}

	if attr.MapsEqual(newObjEnvVars, oldObjEnvVars) && attr.MapsEqual(newLabels, oldLabels) {
		return nil
	}

	// if the updated object does not have a NetworkPolicy yet, we create one
	p, err := h.NetworkPolicyHandler.GetPolicyByPodLabels(oldMetaObj.GetNamespace(), oldLabels)
	if err != nil {
		if errors.Is(err, np.ErrNotFound) {
			h.ObjectHandler = object.NewHandler(h.DyanmicClient, newMetaObj)
			err = h.HandleAdd(newMetaObj)
			if err != nil {
				return err
			}
			return nil
		}
		return err
	}

	if !attr.MapsEqual(newLabels, oldLabels) {
		err := h.handleLabelChange(oldLabels, newLabels, p)
		if err != nil {
			return err
		}
	}

	if !attr.MapsEqual(newObjEnvVars, oldObjEnvVars) {
		err := h.handleEnvVarChange(newLabels, newObjEnvVars, p)
		if err != nil {
			return err
		}
	}

	h.ObjectHandler = object.NewHandler(h.DyanmicClient, p)
	err = h.ObjectHandler.Mutate(object.Update)
	if err != nil {
		return err
	}

	log.Printf("NetworkPolicy %s updated for %s \n", p.GetName(), newMetaObj.GetName())
	return nil
}

/*
HandleDelete handles the case when an object of interest is deleted from the cluster. If the object had a NetworkPolicy belonging to it,
the policy gets deleted
*/
func (h *Handler) HandleDelete(obj interface{}) error {
	objLabels, metaObj, err := object.ConvertToMeta(obj)
	if err != nil {
		return err
	}

	// we don't mess around in the kube-system namespace
	if metaObj.GetNamespace() == "kube-system" {
		return errors.New("objects in the kube-system namespace won't be modified")
	}

	p, err := h.NetworkPolicyHandler.GetPolicyByPodLabels(metaObj.GetNamespace(), objLabels)
	if err != nil {
		return err
	}

	h.ObjectHandler = object.NewHandler(h.DyanmicClient, p)
	err = h.ObjectHandler.Mutate(object.Delete)
	if err != nil {
		return err
	}

	log.Printf("NetworkPolicy %s deleted for %s \n", p.GetName(), metaObj.GetName())
	return nil
}
