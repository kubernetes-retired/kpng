package kube2store

import (
	"testing"

	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"sigs.k8s.io/kpng/server/proxystore"
)

func TestServiceEventHandlerTrafficPolicy(t *testing.T) {
	store := proxystore.New()

	handler := serviceEventHandler{
		eventHandler: eventHandler{
			s:         store,
			isSyncSet: true,
			config:    &Config{
				// defaults
			},
		},
	}

	svc := &v1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: "default",
			Name:      "test-svc",
		},
		Spec: v1.ServiceSpec{
			Type: v1.ServiceTypeClusterIP,
		},
	}

	for testIdx, test := range []struct {
		InternalPolicy   v1.ServiceInternalTrafficPolicyType
		ExpectedInternal bool
		ExternalPolicy   v1.ServiceExternalTrafficPolicyType
		ExpectedExternal bool
	}{
		{v1.ServiceInternalTrafficPolicyCluster, false, v1.ServiceExternalTrafficPolicyTypeCluster, false},
		{v1.ServiceInternalTrafficPolicyLocal, true, v1.ServiceExternalTrafficPolicyTypeCluster, false},
		{v1.ServiceInternalTrafficPolicyCluster, false, v1.ServiceExternalTrafficPolicyTypeLocal, true},
		{v1.ServiceInternalTrafficPolicyLocal, true, v1.ServiceExternalTrafficPolicyTypeLocal, true},
	} {
		svc.Spec.InternalTrafficPolicy = ref(test.InternalPolicy)
		svc.Spec.ExternalTrafficPolicy = test.ExternalPolicy

		handler.onChange(svc)

		store.View(0, func(tx *proxystore.Tx) {
			tx.Each(proxystore.Services, func(kv *proxystore.KV) bool {
				if kv.Service.Service.InternalTrafficToLocal != test.ExpectedInternal {
					t.Errorf("test[%d]: internal: expected %v, got %v", testIdx, test.ExpectedInternal, kv.Service.Service.InternalTrafficToLocal)
				}
				if kv.Service.Service.ExternalTrafficToLocal != test.ExpectedExternal {
					t.Errorf("test[%d]: external: expected %v, got %v", testIdx, test.ExpectedExternal, kv.Service.Service.ExternalTrafficToLocal)
				}
				return true
			})
		})
	}
}

func ref[T any](v T) *T {
	return &v
}
