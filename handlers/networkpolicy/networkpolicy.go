package networkpolicy

import (
	"context"
	"errors"
	"fmt"

	"github.com/adykaaa/k8s-netpol-ctrl/handlers/attribute"
	networkingv1 "k8s.io/api/networking/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

var (
	ErrAlreadyExists = errors.New("a policy with this name already exists")
	ErrEmptyParam    = errors.New("a required parameter is empty")
	ErrNotFound      = errors.New("policy not found")

	//DefaultLabels contains the supported ingress and DNS pod labels. These are by default there in every NetworkPolicy.
	_DefaultLabels = map[string]map[string]string{
		"nginx":   {"app.kubernetes.io/name": "ingress-nginx"},
		"contour": {"app.kubernetes.io/name": "contour"},
		"traefik": {"app.kubernetes.io/name": "traefik"},
		"haproxy": {"app.kubernetes.io/name": "haproxy"},
		"coredns": {"k8s-app": "kube-dns"},
	}
)

type Handler struct {
	Client kubernetes.Interface
}

func isInOldLabels(req metav1.LabelSelectorRequirement, oldLabels map[string][]string) bool {
	oldVals, ok := oldLabels[req.Key]
	if !ok {
		return false
	}
	for _, oldVal := range oldVals {
		for _, reqVal := range req.Values {
			if oldVal == reqVal {
				return true
			}
		}
	}
	return false
}

func updateIngressRules(oldLabels map[string][]string,
	ingressRules []networkingv1.NetworkPolicyIngressRule) {
	for _, rule := range ingressRules {
		updatedIngress := []metav1.LabelSelectorRequirement{}
		if rule.From[0].PodSelector != nil {
			for _, req := range rule.From[0].PodSelector.MatchExpressions {
				if !isInOldLabels(req, oldLabels) {
					updatedIngress = append(updatedIngress, req)
				}
			}
			rule.From[0].PodSelector.MatchExpressions = updatedIngress
		}
	}
}

func updateEgressRules(oldLabels map[string][]string,
	egressRules []networkingv1.NetworkPolicyEgressRule) {
	for _, rule := range egressRules {
		updatedEgress := []metav1.LabelSelectorRequirement{}
		if rule.To[0].PodSelector != nil {
			for _, req := range rule.To[0].PodSelector.MatchExpressions {
				if !isInOldLabels(req, oldLabels) {
					updatedEgress = append(updatedEgress, req)
				}
			}
			rule.To[0].PodSelector.MatchExpressions = updatedEgress
		}
	}
}

func RemoveOldLabels(oldLabels map[string][]string, ingressRules []networkingv1.NetworkPolicyIngressRule, egressRules []networkingv1.NetworkPolicyEgressRule) {
	updateIngressRules(oldLabels, ingressRules)
	updateEgressRules(oldLabels, egressRules)
}

// getDefaultSupportedPeers appends the necessary podSelectors based on the default labels
func getDefaultSupportedPeers(defaultLabels map[string]map[string]string) []networkingv1.NetworkPolicyPeer {
	if len(defaultLabels) == 0 {
		return nil
	}

	keyValueMap := make(map[string][]string)
	for _, labels := range defaultLabels {
		for k, v := range labels {
			keyValueMap[k] = append(keyValueMap[k], v)
		}
	}

	supportedPeers := make([]networkingv1.NetworkPolicyPeer, 0, len(keyValueMap))
	for k, v := range keyValueMap {
		supportedPeers = append(supportedPeers, networkingv1.NetworkPolicyPeer{
			PodSelector: &metav1.LabelSelector{
				MatchExpressions: []metav1.LabelSelectorRequirement{
					{
						Key:      k,
						Operator: metav1.LabelSelectorOpIn,
						Values:   v,
					},
				},
			},
		})
	}
	return supportedPeers
}

/*
ExtendPeers loops through the ingress and egress peers of an existing policy, and appends all the elements from
ingressPol and egressPol to the existing NetworkPolicyPeers.
*/
func ExtendPeers(ingressPol []networkingv1.NetworkPolicyPeer, egressPol []networkingv1.NetworkPolicyPeer, p *networkingv1.NetworkPolicy) error {
	for _, pol := range ingressPol {
		for i := range p.Spec.Ingress {
			if !attribute.Contains(p.Spec.Ingress[i].From, pol) {
				p.Spec.Ingress[i].From = append(p.Spec.Ingress[i].From, pol)
			}
		}
	}

	for _, pol := range egressPol {
		for i := range p.Spec.Egress {
			if !attribute.Contains(p.Spec.Egress[i].To, pol) {
				p.Spec.Egress[i].To = append(p.Spec.Egress[i].To, pol)
			}
		}
	}
	return nil
}

/*
appendLabelsToPeers appends the default supported labels (such as Ingress controller pod labels and DNS pod labels which come from DefaultLabels)
and targetedPodLabels one by one to ingressPeers and egressPeers.
*/
func (h *Handler) AppendLabelsToPeers(targetPodLabels map[string][]string) (ingressPeers []networkingv1.NetworkPolicyPeer, egressPeers []networkingv1.NetworkPolicyPeer, err error) {
	size := len(targetPodLabels) + len(_DefaultLabels)
	ingressPeers = make([]networkingv1.NetworkPolicyPeer, 0, size)
	egressPeers = make([]networkingv1.NetworkPolicyPeer, 0, size)

	defaultPeers := getDefaultSupportedPeers(_DefaultLabels)
	if len(targetPodLabels) == 0 || len(defaultPeers) == 0 {
		return nil, nil, ErrEmptyParam
	}

	for _, ip := range defaultPeers {
		ingressPeers = append(ingressPeers, ip)
		egressPeers = append(egressPeers, ip)
	}

	for k, values := range targetPodLabels {
		// we only want to add a value once to a particular key in MatchExpressions.
		uniqueValues := make(map[string]struct{})
		for _, v := range values {
			uniqueValues[v] = struct{}{}
		}

		var finalValues []string
		for v := range uniqueValues {
			finalValues = append(finalValues, v)
		}

		ingressPeers = append(ingressPeers, networkingv1.NetworkPolicyPeer{
			PodSelector: &metav1.LabelSelector{
				MatchExpressions: []metav1.LabelSelectorRequirement{
					{
						Key:      k,
						Operator: metav1.LabelSelectorOpIn,
						Values:   finalValues,
					},
				},
			},
		})

		egressPeers = append(egressPeers, networkingv1.NetworkPolicyPeer{
			PodSelector: &metav1.LabelSelector{
				MatchExpressions: []metav1.LabelSelectorRequirement{
					{
						Key:      k,
						Operator: metav1.LabelSelectorOpIn,
						Values:   finalValues,
					},
				},
			},
		})
	}

	return ingressPeers, egressPeers, nil
}

/*
NewPolicy deploys a NetworkPolicy which only allows incoming/outgoing communication from pods with the same label,
and to/from objects that have the labels located in the global variable DefaultLabels.
*/
func (h *Handler) NewPolicy(name string, namespace string, podSelectorLabels map[string]string, targetPodLabels map[string][]string) (*networkingv1.NetworkPolicy, error) {
	if name == "" || namespace == "" || len(podSelectorLabels) == 0 {
		return nil, ErrEmptyParam
	}

	ingressPeers, egressPeers, err := h.AppendLabelsToPeers(targetPodLabels)
	if err != nil {
		return nil, err
	}

	policy := &networkingv1.NetworkPolicy{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
		Spec: networkingv1.NetworkPolicySpec{
			PodSelector: metav1.LabelSelector{
				MatchLabels: podSelectorLabels,
			},
			Ingress: []networkingv1.NetworkPolicyIngressRule{
				{
					From: ingressPeers,
				},
			},
			Egress: []networkingv1.NetworkPolicyEgressRule{
				{
					To: egressPeers,
				},
			},
			PolicyTypes: []networkingv1.PolicyType{
				networkingv1.PolicyTypeIngress,
				networkingv1.PolicyTypeEgress,
			},
		},
	}

	return policy, nil
}

// GetPolicyByPodLabels returns the policy where podLabels match LabelSelectorRequirements with LabelSelectorOpIn
func (h *Handler) GetPolicyByPodLabels(namespace string, podLabels map[string]string) (*networkingv1.NetworkPolicy, error) {
	allPolicies, err := h.Client.NetworkingV1().NetworkPolicies(namespace).List(context.Background(), metav1.ListOptions{})
	if err != nil {
		return nil, fmt.Errorf("error retrieving policy list: %w", err)
	}

	if len(allPolicies.Items) == 0 {
		return nil, ErrNotFound
	}

	for _, policy := range allPolicies.Items {
		for _, rule := range policy.Spec.Ingress {
			for _, peer := range rule.From {
				for _, req := range peer.PodSelector.MatchExpressions {
					if req.Operator != metav1.LabelSelectorOpIn {
						continue
					}
					for key, value := range podLabels {
						if key == req.Key && attribute.Contains(req.Values, value) {
							return &policy, nil
						}
					}
				}
			}
		}
		for _, rule := range policy.Spec.Egress {
			for _, peer := range rule.To {
				for _, req := range peer.PodSelector.MatchExpressions {
					if req.Operator != metav1.LabelSelectorOpIn {
						continue
					}
					for key, value := range podLabels {
						if key == req.Key && attribute.Contains(req.Values, value) {
							return &policy, nil
						}
					}
				}
			}
		}
	}
	return nil, ErrNotFound
}
