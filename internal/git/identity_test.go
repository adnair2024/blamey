package git

import "testing"

func TestResolverMerging(t *testing.T) {
	r := NewResolver(false)
	r.Add("Alice", "Alice@Example.com")
	r.Add("Alice", "alice@example.com")
	r.Add("alice w", "alice@example.com")
	r.Add("Gh", "12345+gh@users.noreply.github.com")
	r.Add("Gh", "gh@users.noreply.github.com")
	r.Add("Alice Again", "other@example.com") // same name but different email stays separate
	r.Add("dependabot[bot]", "49699333+dependabot[bot]@users.noreply.github.com")

	a1 := r.Resolve("Alice", "Alice@Example.com")
	a2 := r.Resolve("alice w", "alice@example.com")
	if a1.ID != a2.ID || a1.Name != "Alice" {
		t.Errorf("email case merge failed: %+v %+v", a1, a2)
	}
	g1 := r.Resolve("Gh", "12345+gh@users.noreply.github.com")
	g2 := r.Resolve("Gh", "gh@users.noreply.github.com")
	if g1.ID != g2.ID {
		t.Errorf("noreply merge failed: %+v %+v", g1, g2)
	}
	if r.Resolve("Alice Again", "other@example.com").ID == a1.ID {
		t.Error("distinct emails merged without merge-by-name")
	}
	if !r.Resolve("dependabot[bot]", "49699333+dependabot[bot]@users.noreply.github.com").Bot {
		t.Error("bot not tagged")
	}
	if a1.Bot {
		t.Error("human tagged as bot")
	}
}

func TestResolverMergeByName(t *testing.T) {
	r := NewResolver(true)
	r.Add("Jane  Doe", "jane@home.com")
	r.Add("jane doe", "jane@work.com")
	r.Add("John", "john@x.com")
	if r.Resolve("Jane  Doe", "jane@home.com").ID != r.Resolve("jane doe", "jane@work.com").ID {
		t.Error("same normalized name should merge")
	}
	if r.Resolve("John", "john@x.com").ID == r.Resolve("jane doe", "jane@work.com").ID {
		t.Error("different names merged")
	}
}

func TestResolverDeterministicID(t *testing.T) {
	a := NewResolver(true)
	a.Add("X", "b@x.com")
	a.Add("X", "a@x.com")
	b := NewResolver(true)
	b.Add("X", "a@x.com")
	b.Add("X", "b@x.com")
	if a.Resolve("X", "a@x.com").ID != b.Resolve("X", "b@x.com").ID {
		t.Error("ID depends on insertion order")
	}
}
