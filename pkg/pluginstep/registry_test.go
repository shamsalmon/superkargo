package pluginstep

import (
	"testing"

	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	"github.com/akuity/kargo/pkg/promotion"

	kargoext "github.com/shamsalmon/superkargo/api/v1alpha1"
)

func testScheme(t *testing.T) *runtime.Scheme {
	t.Helper()
	scheme := runtime.NewScheme()
	require.NoError(t, kargoext.AddToScheme(scheme))
	return scheme
}

func noopDial(string) (stepClient, error) { return nil, nil }

func TestDynamicRegistryGet(t *testing.T) {
	scheme := testScheme(t)
	steps := []*kargoext.CustomPromotionStep{
		{
			ObjectMeta: metav1.ObjectMeta{Name: "hello-world"},
			Spec: kargoext.CustomPromotionStepSpec{
				Plugin:                "hw-plugin",
				SharePromotionFolder:  true,
				DefaultErrorThreshold: 3,
			},
		},
		{
			ObjectMeta: metav1.ObjectMeta{Name: "defaults"},
			Spec:       kargoext.CustomPromotionStepSpec{},
		},
	}
	builder := fake.NewClientBuilder().WithScheme(scheme)
	for _, s := range steps {
		builder = builder.WithObjects(s)
	}
	cl := builder.Build()

	var builtinCalled bool
	delegate := promotion.MustNewStepRunnerRegistry(promotion.StepRunnerRegistration{
		Name: "builtin",
		Value: func(promotion.StepRunnerCapabilities) promotion.StepRunner {
			builtinCalled = true
			return nil
		},
	})

	reg := &DynamicRegistry{delegate: delegate, client: cl, dial: noopDial}

	t.Run("delegate hit takes precedence", func(t *testing.T) {
		got, err := reg.Get("builtin")
		require.NoError(t, err)
		require.Equal(t, "builtin", got.Name)
		got.Value(promotion.StepRunnerCapabilities{})
		require.True(t, builtinCalled)
	})

	t.Run("custom step fallback", func(t *testing.T) {
		got, err := reg.Get("hello-world")
		require.NoError(t, err)
		require.Equal(t, "hello-world", got.Name)
		require.Equal(t, uint32(3), got.Metadata.DefaultErrorThreshold)
		require.Empty(t, got.Metadata.RequiredCapabilities)

		runner, ok := got.Value(promotion.StepRunnerCapabilities{}).(*pluginRunner)
		require.True(t, ok)
		require.Equal(t, "hw-plugin", runner.pluginName)
		require.True(t, runner.shareFolder)
	})

	t.Run("plugin name and threshold default", func(t *testing.T) {
		got, err := reg.Get("defaults")
		require.NoError(t, err)
		require.Equal(t, uint32(1), got.Metadata.DefaultErrorThreshold)

		runner := got.Value(promotion.StepRunnerCapabilities{}).(*pluginRunner)
		require.Equal(t, "defaults", runner.pluginName, "defaults to resource name")
		require.False(t, runner.shareFolder)
	})

	t.Run("unknown kind returns error", func(t *testing.T) {
		_, err := reg.Get("nope")
		require.Error(t, err)
	})
}
