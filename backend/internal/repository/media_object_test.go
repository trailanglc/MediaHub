package repository_test

import (
	"context"
	"os"
	"testing"

	"github.com/anhtuanlc/mediahub/internal/platform/postgres"
	"github.com/anhtuanlc/mediahub/internal/repository"
	"github.com/google/uuid"
)

func testPool(t *testing.T) *repository.MediaObjectRepository {
	t.Helper()
	dsn := os.Getenv("DB_DSN")
	if dsn == "" {
		dsn = "postgres://mediahub:mediahub@localhost:5432/mediahub?sslmode=disable"
	}
	pool, err := postgres.NewPool(context.Background(), dsn)
	if err != nil {
		t.Skipf("postgres unavailable: %v", err)
	}
	return repository.NewMediaObjectRepository(pool)
}

// purgeTestSubtree removes integration-test folders from the dev DB (soft + hard delete).
func purgeTestSubtree(t *testing.T, repo *repository.MediaObjectRepository, ancestorID int64) {
	t.Helper()
	t.Cleanup(func() {
		ctx := context.Background()
		_, _ = repo.SoftDeleteSubtree(ctx, ancestorID, 1)
		_, _ = repo.HardDeleteSubtree(ctx, ancestorID)
	})
}

func TestClosureInsertAndList(t *testing.T) {
	repo := testPool(t)
	ctx := context.Background()

	root, err := repo.GetByPublicID(ctx, uuid.MustParse(repository.DefaultRootFolderPublicID))
	if err != nil {
		t.Fatalf("root: %v", err)
	}

	folder, err := repo.CreateWithClosure(ctx, repository.CreateMediaObjectInput{
		PublicID:  uuid.New(),
		ParentID:  &root.ID,
		Type:      "folder",
		Name:      "test-closure-" + uuid.New().String()[:8],
		SizeBytes: 0,
		CreatedBy: 1,
	})
	if err != nil {
		t.Fatalf("create folder: %v", err)
	}
	purgeTestSubtree(t, repo, folder.ID)

	child, err := repo.CreateWithClosure(ctx, repository.CreateMediaObjectInput{
		PublicID:  uuid.New(),
		ParentID:  &folder.ID,
		Type:      "folder",
		Name:      "child",
		SizeBytes: 0,
		CreatedBy: 1,
	})
	if err != nil {
		t.Fatalf("create child: %v", err)
	}

	ancestors, err := repo.GetAncestors(ctx, child.ID)
	if err != nil {
		t.Fatalf("ancestors: %v", err)
	}
	if len(ancestors) < 2 {
		t.Fatalf("expected at least root+parent, got %d", len(ancestors))
	}

	list, err := repo.ListChildren(ctx, folder.ID, repository.ObjectListFilter{Limit: 10})
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	found := false
	for _, item := range list {
		if item.ID == child.ID {
			found = true
		}
	}
	if !found {
		t.Fatal("child not listed under parent")
	}

	_ = repo.SoftDelete(ctx, child.ID, 1)
	_ = repo.SoftDelete(ctx, folder.ID, 1)
}

func TestSoftDeleteSubtree(t *testing.T) {
	repo := testPool(t)
	ctx := context.Background()

	root, err := repo.GetByPublicID(ctx, uuid.MustParse(repository.DefaultRootFolderPublicID))
	if err != nil {
		t.Fatalf("root: %v", err)
	}

	folder, err := repo.CreateWithClosure(ctx, repository.CreateMediaObjectInput{
		PublicID:  uuid.New(),
		ParentID:  &root.ID,
		Type:      "folder",
		Name:      "subtree-" + uuid.New().String()[:8],
		SizeBytes: 0,
		CreatedBy: 1,
	})
	if err != nil {
		t.Fatalf("create folder: %v", err)
	}
	purgeTestSubtree(t, repo, folder.ID)

	child, err := repo.CreateWithClosure(ctx, repository.CreateMediaObjectInput{
		PublicID:  uuid.New(),
		ParentID:  &folder.ID,
		Type:      "folder",
		Name:      "nested",
		SizeBytes: 0,
		CreatedBy: 1,
	})
	if err != nil {
		t.Fatalf("create child: %v", err)
	}

	before, err := repo.ListActiveInSubtree(ctx, folder.ID)
	if err != nil {
		t.Fatalf("list subtree: %v", err)
	}
	if len(before) != 2 {
		t.Fatalf("expected 2 active in subtree, got %d", len(before))
	}
	foundChild := false
	for _, o := range before {
		if o.ID == child.ID {
			foundChild = true
		}
	}
	if !foundChild {
		t.Fatal("nested child missing from subtree list")
	}

	n, err := repo.SoftDeleteSubtree(ctx, folder.ID, 1)
	if err != nil {
		t.Fatalf("soft delete subtree: %v", err)
	}
	if n != 2 {
		t.Fatalf("expected 2 deleted rows, got %d", n)
	}

	list, err := repo.ListChildren(ctx, folder.ID, repository.ObjectListFilter{Limit: 10})
	if err != nil {
		t.Fatalf("list children: %v", err)
	}
	if len(list) != 0 {
		t.Fatalf("expected no active children after subtree delete, got %d", len(list))
	}
	after, err := repo.ListActiveInSubtree(ctx, folder.ID)
	if err != nil {
		t.Fatalf("list subtree after: %v", err)
	}
	if len(after) != 0 {
		t.Fatalf("expected empty subtree after delete, got %d", len(after))
	}
}

func TestRestoreSubtree(t *testing.T) {
	repo := testPool(t)
	ctx := context.Background()

	root, err := repo.GetByPublicID(ctx, uuid.MustParse(repository.DefaultRootFolderPublicID))
	if err != nil {
		t.Fatalf("root: %v", err)
	}

	folder, err := repo.CreateWithClosure(ctx, repository.CreateMediaObjectInput{
		PublicID:  uuid.New(),
		ParentID:  &root.ID,
		Type:      "folder",
		Name:      "restore-" + uuid.New().String()[:8],
		SizeBytes: 0,
		CreatedBy: 1,
	})
	if err != nil {
		t.Fatalf("create folder: %v", err)
	}
	purgeTestSubtree(t, repo, folder.ID)

	child, err := repo.CreateWithClosure(ctx, repository.CreateMediaObjectInput{
		PublicID:  uuid.New(),
		ParentID:  &folder.ID,
		Type:      "folder",
		Name:      "nested-restore",
		SizeBytes: 0,
		CreatedBy: 1,
	})
	if err != nil {
		t.Fatalf("create child: %v", err)
	}

	n, err := repo.SoftDeleteSubtree(ctx, folder.ID, 1)
	if err != nil {
		t.Fatalf("soft delete subtree: %v", err)
	}
	if n != 2 {
		t.Fatalf("expected 2 deleted, got %d", n)
	}

	restored, err := repo.RestoreSubtree(ctx, folder.ID, 1)
	if err != nil {
		t.Fatalf("restore subtree: %v", err)
	}
	if restored != 2 {
		t.Fatalf("expected 2 restored, got %d", restored)
	}

	active, err := repo.ListActiveInSubtree(ctx, folder.ID)
	if err != nil {
		t.Fatalf("list active subtree: %v", err)
	}
	if len(active) != 2 {
		t.Fatalf("expected 2 active after restore, got %d", len(active))
	}
	foundChild := false
	for _, o := range active {
		if o.ID == child.ID {
			foundChild = true
		}
	}
	if !foundChild {
		t.Fatal("nested child not restored")
	}
}

func TestListDeletedRoots(t *testing.T) {
	repo := testPool(t)
	ctx := context.Background()

	root, err := repo.GetByPublicID(ctx, uuid.MustParse(repository.DefaultRootFolderPublicID))
	if err != nil {
		t.Fatalf("root: %v", err)
	}

	folder, err := repo.CreateWithClosure(ctx, repository.CreateMediaObjectInput{
		PublicID:  uuid.New(),
		ParentID:  &root.ID,
		Type:      "folder",
		Name:      "trash-roots-" + uuid.New().String()[:8],
		SizeBytes: 0,
		CreatedBy: 1,
	})
	if err != nil {
		t.Fatalf("create folder: %v", err)
	}
	purgeTestSubtree(t, repo, folder.ID)

	child, err := repo.CreateWithClosure(ctx, repository.CreateMediaObjectInput{
		PublicID:  uuid.New(),
		ParentID:  &folder.ID,
		Type:      "file",
		Name:      "nested.txt",
		SizeBytes: 1,
		CreatedBy: 1,
	})
	if err != nil {
		t.Fatalf("create child: %v", err)
	}

	standalone, err := repo.CreateWithClosure(ctx, repository.CreateMediaObjectInput{
		PublicID:  uuid.New(),
		ParentID:  &root.ID,
		Type:      "file",
		Name:      "solo-" + uuid.New().String()[:8] + ".txt",
		SizeBytes: 1,
		CreatedBy: 1,
	})
	if err != nil {
		t.Fatalf("create standalone: %v", err)
	}

	if _, err := repo.SoftDeleteSubtree(ctx, folder.ID, 1); err != nil {
		t.Fatalf("soft delete folder: %v", err)
	}
	if err := repo.SoftDelete(ctx, standalone.ID, 1); err != nil {
		t.Fatalf("soft delete standalone: %v", err)
	}

	allDeleted, err := repo.ListDeleted(ctx, repository.ObjectListFilter{Limit: 100})
	if err != nil {
		t.Fatalf("list deleted: %v", err)
	}
	if len(allDeleted) < 3 {
		t.Fatalf("expected at least 3 deleted rows (folder+child+standalone), got %d", len(allDeleted))
	}

	roots, err := repo.ListDeletedRoots(ctx, repository.ObjectListFilter{Limit: 100})
	if err != nil {
		t.Fatalf("list deleted roots: %v", err)
	}

	foundFolder := false
	foundStandalone := false
	for _, o := range roots {
		if o.ID == child.ID {
			t.Fatal("nested deleted file must not appear as trash root")
		}
		if o.ID == folder.ID {
			foundFolder = true
		}
		if o.ID == standalone.ID {
			foundStandalone = true
		}
	}
	if !foundFolder || !foundStandalone {
		t.Fatalf("expected folder and standalone in trash roots, folder=%v solo=%v", foundFolder, foundStandalone)
	}

	if _, err := repo.HardDeleteSubtree(ctx, folder.ID); err != nil {
		t.Fatalf("purge folder: %v", err)
	}
	if _, err := repo.HardDeleteSubtree(ctx, standalone.ID); err != nil {
		t.Fatalf("purge standalone: %v", err)
	}

	empty, err := repo.ListDeletedRoots(ctx, repository.ObjectListFilter{Limit: 10})
	if err != nil {
		t.Fatalf("list after purge: %v", err)
	}
	for _, o := range empty {
		if o.ID == folder.ID || o.ID == standalone.ID || o.ID == child.ID {
			t.Fatalf("trash should be empty after purge, still has id=%d", o.ID)
		}
	}
}

func TestSearchActiveInSubtree(t *testing.T) {
	repo := testPool(t)
	ctx := context.Background()

	root, err := repo.GetByPublicID(ctx, uuid.MustParse(repository.DefaultRootFolderPublicID))
	if err != nil {
		t.Fatalf("root: %v", err)
	}

	parent, err := repo.CreateWithClosure(ctx, repository.CreateMediaObjectInput{
		PublicID:  uuid.New(),
		ParentID:  &root.ID,
		Type:      "folder",
		Name:      "subtree-search-" + uuid.New().String()[:8],
		SizeBytes: 0,
		CreatedBy: 1,
	})
	if err != nil {
		t.Fatalf("create parent: %v", err)
	}
	purgeTestSubtree(t, repo, parent.ID)

	childFolder, err := repo.CreateWithClosure(ctx, repository.CreateMediaObjectInput{
		PublicID:  uuid.New(),
		ParentID:  &parent.ID,
		Type:      "folder",
		Name:      "nested-folder",
		SizeBytes: 0,
		CreatedBy: 1,
	})
	if err != nil {
		t.Fatalf("create child folder: %v", err)
	}

	deepFile, err := repo.CreateWithClosure(ctx, repository.CreateMediaObjectInput{
		PublicID:  uuid.New(),
		ParentID:  &childFolder.ID,
		Type:      "file",
		Name:      "deep-unique-file.txt",
		SizeBytes: 1,
		CreatedBy: 1,
	})
	if err != nil {
		t.Fatalf("create deep file: %v", err)
	}

	shallow, err := repo.ListChildren(ctx, parent.ID, repository.ObjectListFilter{
		Query: "deep-unique",
		Limit: 10,
	})
	if err != nil {
		t.Fatalf("list children search: %v", err)
	}
	if len(shallow) != 0 {
		t.Fatalf("ListChildren must not find nested file, got %d", len(shallow))
	}

	found, err := repo.SearchActiveInSubtree(ctx, parent.ID, repository.ObjectListFilter{
		Query: "deep-unique",
		Limit: 10,
	})
	if err != nil {
		t.Fatalf("subtree search: %v", err)
	}
	if len(found) != 1 || found[0].ID != deepFile.ID {
		t.Fatalf("expected deep file, got %d items", len(found))
	}
}

func TestSearchAccentInsensitive(t *testing.T) {
	repo := testPool(t)
	ctx := context.Background()

	root, err := repo.GetByPublicID(ctx, uuid.MustParse(repository.DefaultRootFolderPublicID))
	if err != nil {
		t.Fatalf("root: %v", err)
	}

	file, err := repo.CreateWithClosure(ctx, repository.CreateMediaObjectInput{
		PublicID:  uuid.New(),
		ParentID:  &root.ID,
		Type:      "file",
		Name:      "Ảnh-" + uuid.New().String()[:8],
		SizeBytes: 1,
		CreatedBy: 1,
	})
	if err != nil {
		t.Fatalf("create file: %v", err)
	}
	t.Cleanup(func() {
		_, _ = repo.SoftDeleteSubtree(ctx, file.ID, 1)
		_, _ = repo.HardDeleteSubtree(ctx, file.ID)
	})

	found, err := repo.SearchActiveByName(ctx, repository.ObjectListFilter{
		Query: "anh",
		Limit: 50,
	})
	if err != nil {
		t.Fatalf("search: %v", err)
	}
	matched := false
	for _, o := range found {
		if o.ID == file.ID {
			matched = true
			break
		}
	}
	if !matched {
		t.Fatal("expected accent-insensitive match for query 'anh' on name with 'Ảnh'")
	}
}
