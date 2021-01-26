package hooks

import (
	"context"
	"encoding/json"
	"net/http"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/webhook"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"
)

const (
	annotationKubernetesIngressClass = "kubernetes.io/ingress.class"
	annotationContourIngressClass    = "projectcontour.io/ingress.class"
)

// +kubebuilder:webhook:path=/mutate-projectcontour-io-httpproxy,mutating=true,failurePolicy=fail,sideEffects=None,groups=projectcontour.io,resources=httpproxies,verbs=create,versions=v1,name=mhttpproxy.kb.io,admissionReviewVersions={v1,v1beta1}

type contourHTTPProxyMutator struct {
	client       client.Client
	decoder      *admission.Decoder
	defaultClass string
}

// NewContourHTTPProxyMutator creates a webhook handler for Contour HTTPProxy.
func NewContourHTTPProxyMutator(c client.Client, dec *admission.Decoder, defaultClass string) http.Handler {
	return &webhook.Admission{Handler: &contourHTTPProxyMutator{c, dec, defaultClass}}
}

func (m *contourHTTPProxyMutator) Handle(ctx context.Context, req admission.Request) admission.Response {
	hp := &unstructured.Unstructured{}
	err := m.decoder.Decode(req, hp)
	if err != nil {
		return admission.Errored(http.StatusBadRequest, err)
	}
	ann := hp.GetAnnotations()

	if _, ok := ann[annotationKubernetesIngressClass]; ok {
		return admission.Allowed("ok")
	}
	if _, ok := ann[annotationContourIngressClass]; ok {
		return admission.Allowed("ok")
	}

	if ann == nil {
		ann = make(map[string]string)
	}
	ann[annotationKubernetesIngressClass] = m.defaultClass
	hp.SetAnnotations(ann)

	marshaled, err := json.Marshal(hp)
	if err != nil {
		return admission.Errored(http.StatusInternalServerError, err)
	}

	return admission.PatchResponseFromRaw(req.Object.Raw, marshaled)
}
