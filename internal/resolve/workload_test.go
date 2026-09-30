package resolve

import "testing"

func TestReplicaSetGivesItsDeployment(t *testing.T) {
	resolved := Workloads(
		[]Owner{{Namespace: "payments", Child: "api-abc-123", OwnerKind: "ReplicaSet", OwnerName: "api-abc"}},
		[]Owner{{Namespace: "payments", Child: "api-abc", OwnerKind: "Deployment", OwnerName: "api"}},
	)
	got := resolved[Pod{Namespace: "payments", Name: "api-abc-123"}]
	want := Identity{Namespace: "payments", Kind: "Deployment", Name: "api"}
	if got != want {
		t.Errorf("got %+v, want %+v", got, want)
	}
}

func TestDirectOwnersAreKept(t *testing.T) {
	resolved := Workloads(
		[]Owner{
			{Namespace: "payments", Child: "db-0", OwnerKind: "StatefulSet", OwnerName: "db"},
			{Namespace: "ops", Child: "custom-0", OwnerKind: "CustomOwner", OwnerName: "x"},
		},
		nil,
	)
	got := resolved[Pod{Namespace: "payments", Name: "db-0"}]
	want := Identity{Namespace: "payments", Kind: "StatefulSet", Name: "db"}
	if got != want {
		t.Errorf("got %+v, want %+v", got, want)
	}
	got = resolved[Pod{Namespace: "ops", Name: "custom-0"}]
	want = Identity{Namespace: "ops", Kind: "CustomOwner", Name: "x"}
	if got != want {
		t.Errorf("got %+v, want %+v", got, want)
	}
}

func TestJobIsNotRolledUp(t *testing.T) {
	resolved := Workloads(
		[]Owner{{Namespace: "payments", Child: "api-xyz", OwnerKind: "Job", OwnerName: "api"}},
		[]Owner{{Namespace: "payments", Child: "api", OwnerKind: "Deployment", OwnerName: "api"}},
	)
	got := resolved[Pod{Namespace: "payments", Name: "api-xyz"}]
	want := Identity{Namespace: "payments", Kind: "Job", Name: "api"}
	if got != want {
		t.Errorf("got %+v, want %+v", got, want)
	}
}

// TestLastRecordWins pins that, when two records name the same pod, or two
// records name the same replica set, the last one in the list wins: a
// faithful port of the original, whose dict comprehension and loop both
// keep the last assignment to a repeated key.
func TestLastRecordWins(t *testing.T) {
	t.Run("two records name one pod", func(t *testing.T) {
		resolved := Workloads(
			[]Owner{
				{Namespace: "payments", Child: "api-0", OwnerKind: "Deployment", OwnerName: "first"},
				{Namespace: "payments", Child: "api-0", OwnerKind: "Deployment", OwnerName: "second"},
			},
			nil,
		)
		got := resolved[Pod{Namespace: "payments", Name: "api-0"}]
		want := Identity{Namespace: "payments", Kind: "Deployment", Name: "second"}
		if got != want {
			t.Errorf("got %+v, want %+v", got, want)
		}
	})

	t.Run("two records name one replica set", func(t *testing.T) {
		resolved := Workloads(
			[]Owner{{Namespace: "payments", Child: "api-abc-123", OwnerKind: "ReplicaSet", OwnerName: "api-abc"}},
			[]Owner{
				{Namespace: "payments", Child: "api-abc", OwnerKind: "Deployment", OwnerName: "first"},
				{Namespace: "payments", Child: "api-abc", OwnerKind: "Deployment", OwnerName: "second"},
			},
		)
		got := resolved[Pod{Namespace: "payments", Name: "api-abc-123"}]
		want := Identity{Namespace: "payments", Kind: "Deployment", Name: "second"}
		if got != want {
			t.Errorf("got %+v, want %+v", got, want)
		}
	})
}

func TestPodWithoutOwnerStandsAlone(t *testing.T) {
	resolved := Workloads(
		[]Owner{{Namespace: "ops", Child: "debug", OwnerKind: "", OwnerName: ""}},
		nil,
	)
	got := resolved[Pod{Namespace: "ops", Name: "debug"}]
	want := Identity{Namespace: "ops", Kind: KindStandalonePod, Name: "debug"}
	if got != want {
		t.Errorf("got %+v, want %+v", got, want)
	}
}

func TestReplicaSetWithoutOwnerIsKept(t *testing.T) {
	resolved := Workloads(
		[]Owner{
			{Namespace: "payments", Child: "orphan-abc", OwnerKind: "ReplicaSet", OwnerName: "orphan"},
			{Namespace: "payments", Child: "bare-abc", OwnerKind: "ReplicaSet", OwnerName: "bare"},
		},
		[]Owner{{Namespace: "payments", Child: "bare", OwnerKind: "", OwnerName: ""}},
	)
	got := resolved[Pod{Namespace: "payments", Name: "orphan-abc"}]
	want := Identity{Namespace: "payments", Kind: "ReplicaSet", Name: "orphan"}
	if got != want {
		t.Errorf("got %+v, want %+v", got, want)
	}
	got = resolved[Pod{Namespace: "payments", Name: "bare-abc"}]
	want = Identity{Namespace: "payments", Kind: "ReplicaSet", Name: "bare"}
	if got != want {
		t.Errorf("got %+v, want %+v", got, want)
	}
}

func TestReplicaSetIsFoundInItsNamespace(t *testing.T) {
	resolved := Workloads(
		[]Owner{
			{Namespace: "payments", Child: "web-abc-1", OwnerKind: "ReplicaSet", OwnerName: "web-abc"},
			{Namespace: "billing", Child: "web-abc-2", OwnerKind: "ReplicaSet", OwnerName: "web-abc"},
		},
		[]Owner{
			{Namespace: "payments", Child: "web-abc", OwnerKind: "Deployment", OwnerName: "payments-web"},
			{Namespace: "billing", Child: "web-abc", OwnerKind: "Deployment", OwnerName: "billing-web"},
		},
	)
	got := resolved[Pod{Namespace: "payments", Name: "web-abc-1"}]
	want := Identity{Namespace: "payments", Kind: "Deployment", Name: "payments-web"}
	if got != want {
		t.Errorf("got %+v, want %+v", got, want)
	}
	got = resolved[Pod{Namespace: "billing", Name: "web-abc-2"}]
	want = Identity{Namespace: "billing", Kind: "Deployment", Name: "billing-web"}
	if got != want {
		t.Errorf("got %+v, want %+v", got, want)
	}
}

func TestCanonicalKindTable(t *testing.T) {
	cases := []struct {
		value string
		want  string
	}{
		{"deployment", "Deployment"},
		{"STATEFULSET", "StatefulSet"},
		{"job", "Job"},
		{"Rollout", "Rollout"},
		{"rollout", "rollout"},
		{"", ""},
	}
	for _, c := range cases {
		if got := CanonicalKind(c.value); got != c.want {
			t.Errorf("CanonicalKind(%q) = %q, want %q", c.value, got, c.want)
		}
	}
}
