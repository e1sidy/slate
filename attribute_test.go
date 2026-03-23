package slate

import "testing"

func TestDefineAttr(t *testing.T) {
	store := tempDB(t)

	if err := store.DefineAttr(ctx, "env", AttrString, "Environment"); err != nil {
		t.Fatal(err)
	}

	def, err := store.GetAttrDef(ctx, "env")
	if err != nil {
		t.Fatal(err)
	}
	if def.Key != "env" || def.Type != AttrString {
		t.Errorf("got %s/%s, want env/string", def.Key, def.Type)
	}
}

func TestDefineAttr_Idempotent(t *testing.T) {
	store := tempDB(t)

	store.DefineAttr(ctx, "env", AttrString, "first")
	if err := store.DefineAttr(ctx, "env", AttrString, "second"); err != nil {
		t.Fatal("idempotent define should not error")
	}
}

func TestDefineAttr_InvalidType(t *testing.T) {
	store := tempDB(t)

	if err := store.DefineAttr(ctx, "bad", AttrType("invalid"), ""); err == nil {
		t.Fatal("expected error for invalid type")
	}
}

func TestSetGetAttr(t *testing.T) {
	store := tempDB(t)
	task, _ := store.Create(ctx, CreateParams{Title: "Task"})

	store.DefineAttr(ctx, "env", AttrString, "")
	if err := store.SetAttr(ctx, task.ID, "env", "production"); err != nil {
		t.Fatal(err)
	}

	attr, err := store.GetAttr(ctx, task.ID, "env")
	if err != nil {
		t.Fatal(err)
	}
	if attr.Value != "production" {
		t.Errorf("value = %q, want production", attr.Value)
	}
	if attr.Type != AttrString {
		t.Errorf("type = %q, want string", attr.Type)
	}
}

func TestSetAttr_BooleanValidation(t *testing.T) {
	store := tempDB(t)
	task, _ := store.Create(ctx, CreateParams{Title: "Task"})

	store.DefineAttr(ctx, "active", AttrBoolean, "")

	if err := store.SetAttr(ctx, task.ID, "active", "true"); err != nil {
		t.Fatal(err)
	}
	if err := store.SetAttr(ctx, task.ID, "active", "false"); err != nil {
		t.Fatal(err)
	}
	if err := store.SetAttr(ctx, task.ID, "active", "maybe"); err == nil {
		t.Fatal("expected error for invalid boolean")
	}
}

func TestSetAttr_ObjectValidation(t *testing.T) {
	store := tempDB(t)
	task, _ := store.Create(ctx, CreateParams{Title: "Task"})

	store.DefineAttr(ctx, "config", AttrObject, "")

	if err := store.SetAttr(ctx, task.ID, "config", `{"key":"val"}`); err != nil {
		t.Fatal(err)
	}
	if err := store.SetAttr(ctx, task.ID, "config", "not json"); err == nil {
		t.Fatal("expected error for invalid JSON")
	}
}

func TestSetAttr_UndefinedKey(t *testing.T) {
	store := tempDB(t)
	task, _ := store.Create(ctx, CreateParams{Title: "Task"})

	if err := store.SetAttr(ctx, task.ID, "nonexistent", "val"); err == nil {
		t.Fatal("expected error for undefined key")
	}
}

func TestDeleteAttr(t *testing.T) {
	store := tempDB(t)
	task, _ := store.Create(ctx, CreateParams{Title: "Task"})

	store.DefineAttr(ctx, "env", AttrString, "")
	store.SetAttr(ctx, task.ID, "env", "prod")

	if err := store.DeleteAttr(ctx, task.ID, "env"); err != nil {
		t.Fatal(err)
	}

	_, err := store.GetAttr(ctx, task.ID, "env")
	if err == nil {
		t.Fatal("expected error after delete")
	}
}

func TestListAttrDefs(t *testing.T) {
	store := tempDB(t)

	store.DefineAttr(ctx, "alpha", AttrString, "")
	store.DefineAttr(ctx, "beta", AttrBoolean, "")

	defs, err := store.ListAttrDefs(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(defs) != 2 {
		t.Errorf("count = %d, want 2", len(defs))
	}
}

func TestUndefineAttr(t *testing.T) {
	store := tempDB(t)
	task, _ := store.Create(ctx, CreateParams{Title: "Task"})

	store.DefineAttr(ctx, "env", AttrString, "")
	store.SetAttr(ctx, task.ID, "env", "prod")

	if err := store.UndefineAttr(ctx, "env"); err != nil {
		t.Fatal(err)
	}

	defs, _ := store.ListAttrDefs(ctx)
	if len(defs) != 0 {
		t.Errorf("defs count = %d after undefine, want 0", len(defs))
	}
}

func TestAttrs(t *testing.T) {
	store := tempDB(t)
	task, _ := store.Create(ctx, CreateParams{Title: "Task"})

	store.DefineAttr(ctx, "a", AttrString, "")
	store.DefineAttr(ctx, "b", AttrBoolean, "")
	store.SetAttr(ctx, task.ID, "a", "hello")
	store.SetAttr(ctx, task.ID, "b", "true")

	attrs, err := store.Attrs(ctx, task.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(attrs) != 2 {
		t.Errorf("attrs count = %d, want 2", len(attrs))
	}
}

func TestGetFull(t *testing.T) {
	store := tempDB(t)
	task, _ := store.Create(ctx, CreateParams{Title: "Full task"})

	store.DefineAttr(ctx, "env", AttrString, "")
	store.SetAttr(ctx, task.ID, "env", "staging")

	full, err := store.GetFull(ctx, task.ID)
	if err != nil {
		t.Fatal(err)
	}
	if full.Attrs["env"] != "staging" {
		t.Errorf("attrs[env] = %q, want staging", full.Attrs["env"])
	}
}
