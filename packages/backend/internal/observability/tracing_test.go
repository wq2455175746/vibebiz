package observability

import "testing"

func TestSamplerFromEnv(t *testing.T) {
	t.Setenv("OTEL_TRACES_SAMPLER_ARG", "0.25")

	cases := []struct {
		name string
		env  string
		want string
	}{
		{"always_on", "always_on", "AlwaysOnSampler"},
		{"always_off", "always_off", "AlwaysOffSampler"},
		{"traceidratio", "traceidratio", "TraceIDRatioBased{0.25}"},
		{"parentbased_always_on", "parentbased_always_on", "ParentBased{root:AlwaysOnSampler,remoteParentSampled:AlwaysOnSampler,remoteParentNotSampled:AlwaysOffSampler,localParentSampled:AlwaysOnSampler,localParentNotSampled:AlwaysOffSampler}"},
		{"parentbased_always_off", "parentbased_always_off", "ParentBased{root:AlwaysOffSampler,remoteParentSampled:AlwaysOnSampler,remoteParentNotSampled:AlwaysOffSampler,localParentSampled:AlwaysOnSampler,localParentNotSampled:AlwaysOffSampler}"},
		{"parentbased_traceidratio", "parentbased_traceidratio", "ParentBased{root:TraceIDRatioBased{0.25},remoteParentSampled:AlwaysOnSampler,remoteParentNotSampled:AlwaysOffSampler,localParentSampled:AlwaysOnSampler,localParentNotSampled:AlwaysOffSampler}"},
		{"default", "", "ParentBased{root:AlwaysOnSampler,remoteParentSampled:AlwaysOnSampler,remoteParentNotSampled:AlwaysOffSampler,localParentSampled:AlwaysOnSampler,localParentNotSampled:AlwaysOffSampler}"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Setenv("OTEL_TRACES_SAMPLER", tc.env)
			if got := samplerFromEnv().Description(); got != tc.want {
				t.Fatalf("sampler = %q, want %q", got, tc.want)
			}
		})
	}
}

func TestSamplerRatioDefaultsWhenArgInvalid(t *testing.T) {
	t.Setenv("OTEL_TRACES_SAMPLER", "traceidratio")
	t.Setenv("OTEL_TRACES_SAMPLER_ARG", "not-a-number")

	if got := samplerFromEnv().Description(); got != "AlwaysOnSampler" {
		t.Fatalf("sampler = %q, want ratio 1.0 to collapse to AlwaysOnSampler", got)
	}
}
