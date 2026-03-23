package slate

import "testing"

func TestSearch(t *testing.T) {
	store := tempDB(t)

	store.Create(ctx, CreateParams{Title: "Fix authentication bug", Description: "JWT token expiry"})
	store.Create(ctx, CreateParams{Title: "Add feature", Description: "New dashboard"})

	results, err := store.Search(ctx, "auth")
	if err != nil {
		t.Fatal(err)
	}
	if len(results) != 1 {
		t.Errorf("search results = %d, want 1", len(results))
	}

	results, err = store.Search(ctx, "dashboard")
	if err != nil {
		t.Fatal(err)
	}
	if len(results) != 1 {
		t.Errorf("search by desc = %d, want 1", len(results))
	}
}

func TestGetTree(t *testing.T) {
	store := tempDB(t)

	parent, _ := store.Create(ctx, CreateParams{Title: "Epic"})
	store.Create(ctx, CreateParams{Title: "Child 1", ParentID: parent.ID})
	child2, _ := store.Create(ctx, CreateParams{Title: "Child 2", ParentID: parent.ID})
	store.Create(ctx, CreateParams{Title: "Grandchild", ParentID: child2.ID})

	tree, err := store.GetTree(ctx, parent.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(tree.Children) != 2 {
		t.Errorf("children = %d, want 2", len(tree.Children))
	}
	// Find child2 and check grandchild.
	for _, c := range tree.Children {
		if c.Title == "Child 2" && len(c.Children) != 1 {
			t.Errorf("grandchildren of Child 2 = %d, want 1", len(c.Children))
		}
	}
}

func TestList_Filters(t *testing.T) {
	store := tempDB(t)

	store.Create(ctx, CreateParams{Title: "Bug", Type: TypeBug, Priority: P1})
	store.Create(ctx, CreateParams{Title: "Feature", Type: TypeFeature, Priority: P3})

	bug := TypeBug
	tasks, err := store.List(ctx, ListParams{Type: &bug})
	if err != nil {
		t.Fatal(err)
	}
	if len(tasks) != 1 || tasks[0].Title != "Bug" {
		t.Errorf("type filter: got %d tasks", len(tasks))
	}

	p1 := P1
	tasks, err = store.List(ctx, ListParams{Priority: &p1})
	if err != nil {
		t.Fatal(err)
	}
	if len(tasks) != 1 || tasks[0].Title != "Bug" {
		t.Errorf("priority filter: got %d tasks", len(tasks))
	}
}

func TestList_Pagination(t *testing.T) {
	store := tempDB(t)

	for i := 0; i < 5; i++ {
		store.Create(ctx, CreateParams{Title: "Task"})
	}

	tasks, err := store.List(ctx, ListParams{Limit: 2})
	if err != nil {
		t.Fatal(err)
	}
	if len(tasks) != 2 {
		t.Errorf("limit 2: got %d", len(tasks))
	}

	tasks, err = store.List(ctx, ListParams{Limit: 2, Offset: 3})
	if err != nil {
		t.Fatal(err)
	}
	if len(tasks) != 2 {
		t.Errorf("limit 2 offset 3: got %d", len(tasks))
	}
}

func TestList_ExcludeStatuses(t *testing.T) {
	store := tempDB(t)

	t1, _ := store.Create(ctx, CreateParams{Title: "Open"})
	t2, _ := store.Create(ctx, CreateParams{Title: "Closed"})
	store.CloseTask(ctx, t2.ID, "done", "tester")

	tasks, err := store.List(ctx, ListParams{
		ExcludeStatuses: []Status{StatusClosed, StatusCancelled},
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(tasks) != 1 || tasks[0].ID != t1.ID {
		t.Errorf("exclude closed: got %d tasks", len(tasks))
	}
}
