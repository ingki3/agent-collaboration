// FR-2A.1a (v0.19.28, Director 결정 2026-10-16) — 「채택(endorse)」 splits
// artifact_submitted(agent: X) into two ways to satisfy it: X submits it
// themselves (E6-01/02 above), or X endorses someone else's submission.
// Both are the SAME kind of event to the tree (actor = X), so this reuses
// the E6 golden table's fixtures rather than inventing new ones.
package sessions

import (
	"testing"
)

func TestCompletionEndorseGolden(t *testing.T) {
	def := and(artifact(agW), userAppr)

	t.Run("designated_agent_endorses_someone_elses_submission", func(t *testing.T) {
		// W never submitted anything themselves — R's submission is what W
		// is endorsing (실사용 「게임 기획」 방: Lead 위임만 하는 구조).
		r := mustApply(t, def, event{Kind: "artifact_endorse", Actor: agW})

		if !hasAtom(r.MetAtoms, "artifact_submitted") {
			t.Errorf("met atoms = %v, want artifact_submitted — endorsing is the SAME kind of event as submitting", r.MetAtoms)
		}
		if !r.HitlIssued {
			t.Fatal("the rest of the tree is now satisfied — the platform issues user_approval (FR-2.2), exactly as a direct submit would")
		}
		if r.HitlSource != "system" {
			t.Errorf("hitl source = %q, want system", r.HitlSource)
		}
	})

	t.Run("wrong_agent_endorsing_is_an_explicit_error_not_a_silent_no_op", func(t *testing.T) {
		// Unlike a plain submit by the wrong agent (silently satisfies
		// nothing — E6-02), endorsing as the wrong agent errors: a person
		// clicking "채택" for someone else should learn it did nothing, the
		// way review_approve/reject does for a non-designated reviewer.
		r := mustApply(t, def, event{Kind: "artifact_endorse", Actor: agR})

		if hasAtom(r.MetAtoms, "artifact_submitted") {
			t.Error("artifact_submitted names W — R endorsing must not satisfy it")
		}
		if r.CLIError == "" {
			t.Error("endorsing as the wrong agent must answer CLIError (→ httpapi 403 not_designated_assignee), not silently do nothing")
		}
		if r.HitlIssued {
			t.Error("no HITL may be issued on a refused endorsement")
		}
	})

	t.Run("endorsing after already satisfied by a direct submit is idempotent", func(t *testing.T) {
		st := State{Met: map[string]bool{CondArtifactSubmitted: true}}
		o := ApplyEvent(toTree(def), st, Event{Kind: "artifact_endorse", Actor: agW})

		if !hasAtomReal(o.MetAtoms, CondArtifactSubmitted) {
			t.Errorf("met atoms = %v, want artifact_submitted to stay met", o.MetAtoms)
		}
		// Nothing CHANGED (it was already met), so this must not re-issue
		// the platform's user_approval request a second time (T-APPROVAL).
		if o.HitlIssued {
			t.Error("endorsing an already-satisfied atom must not be treated as an advance")
		}
	})
}

func hasAtomReal(atoms []string, want string) bool {
	for _, a := range atoms {
		if a == want {
			return true
		}
	}
	return false
}
