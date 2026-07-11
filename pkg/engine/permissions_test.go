package engine

import "testing"

func TestPermissionRulesPersistAsAllowedTools(t *testing.T) {
	dir := t.TempDir()
	store := newPermissionStore(dir)
	rule, err := store.Add("Bash", "git status:*")
	if err != nil {
		t.Fatal(err)
	}
	if got := store.AllowedTools(); len(got) != 1 || got[0] != "Bash(git status:*)" {
		t.Fatalf("allowed tools = %v", got)
	}
	reloaded := newPermissionStore(dir)
	if got := reloaded.AllowedTools(); len(got) != 1 || got[0] != "Bash(git status:*)" {
		t.Fatalf("reloaded allowed tools = %v", got)
	}
	if !reloaded.Delete(rule.RuleID) || len(reloaded.List()) != 0 {
		t.Fatal("permission rule was not deleted")
	}
}

func TestPermissionRuleRejectsUnsafeToolName(t *testing.T) {
	store := newPermissionStore(t.TempDir())
	if _, err := store.Add("Bash) --dangerously-skip-permissions", "*"); err == nil {
		t.Fatal("expected unsafe tool name to be rejected")
	}
}
