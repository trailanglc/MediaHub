package authz_test

import (
	"context"
	"os"
	"testing"

	"github.com/anhtuanlc/mediahub/internal/auth"
	"github.com/anhtuanlc/mediahub/internal/authz"
	"github.com/anhtuanlc/mediahub/internal/platform/postgres"
	"github.com/anhtuanlc/mediahub/internal/repository"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

func testPool(t *testing.T) *pgxpool.Pool {
	t.Helper()
	dsn := os.Getenv("DB_DSN")
	if dsn == "" {
		dsn = "postgres://mediahub:Anhtuanlc.12@localhost:5432/mediahub?sslmode=disable"
	}
	pool, err := postgres.NewPool(context.Background(), dsn)
	if err != nil {
		t.Skipf("postgres not available: %v", err)
	}
	return pool
}

func TestHasPermission_InheritedFromFolder(t *testing.T) {
	pool := testPool(t)
	defer pool.Close()
	ctx := context.Background()

	perms := repository.NewPermissionRepository(pool)
	svc := authz.NewService(perms)
	users := repository.NewUserRepository(pool)
	objects := repository.NewMediaObjectRepository(pool)

	rootPID, _ := uuid.Parse("00000000-0000-4000-8000-000000000001")
	root, err := objects.GetByPublicID(ctx, rootPID)
	if err != nil {
		t.Fatalf("root folder: %v (run migrations)", err)
	}

	hash, err := auth.HashPassword("TestPass123")
	if err != nil {
		t.Fatal(err)
	}
	memberEmail := "authz-test-" + uuid.New().String()[:8] + "@example.com"
	member, err := users.CreateMember(ctx, memberEmail, hash, "viewer", uuid.New(), 1)
	if err != nil {
		t.Fatalf("create member: %v", err)
	}
	defer func() {
		_, _ = pool.Exec(ctx, `DELETE FROM permissions WHERE user_id = $1`, member.ID)
		_, _ = pool.Exec(ctx, `DELETE FROM users WHERE id = $1`, member.ID)
	}()

	if _, err := perms.Grant(ctx, member.ID, root.ID, 1, "folder", string(authz.ActionRead)); err != nil {
		t.Fatalf("grant: %v", err)
	}

	childPID := uuid.New()
	var childID int64
	err = pool.QueryRow(ctx, `
		INSERT INTO media_objects (public_id, parent_id, type, name, status)
		VALUES ($1, $2, 'file', 'child-test', 'active')
		RETURNING id
	`, childPID, root.ID).Scan(&childID)
	if err != nil {
		t.Fatalf("insert child: %v", err)
	}
	defer func() {
		_, _ = pool.Exec(ctx, `DELETE FROM object_paths WHERE descendant_id = $1`, childID)
		_, _ = pool.Exec(ctx, `DELETE FROM media_objects WHERE id = $1`, childID)
	}()

	_, err = pool.Exec(ctx, `
		INSERT INTO object_paths (ancestor_id, descendant_id, depth) VALUES ($1, $1, 0)
		ON CONFLICT DO NOTHING
	`, childID)
	if err != nil {
		t.Fatalf("self path: %v", err)
	}
	_, err = pool.Exec(ctx, `
		INSERT INTO object_paths (ancestor_id, descendant_id, depth) VALUES ($1, $2, 1)
		ON CONFLICT DO NOTHING
	`, root.ID, childID)
	if err != nil {
		t.Fatalf("ancestor path: %v", err)
	}

	ok, err := svc.HasPermission(ctx, member.ID, "viewer", childID, authz.ActionRead)
	if err != nil {
		t.Fatalf("has permission: %v", err)
	}
	if !ok {
		t.Fatal("expected inherited read permission on child file")
	}

	okOwner, err := svc.HasPermission(ctx, member.ID, "owner", childID, authz.ActionDelete)
	if err != nil {
		t.Fatalf("owner check: %v", err)
	}
	if !okOwner {
		t.Fatal("owner role should bypass permission checks")
	}
}

func TestCapabilitiesMap_BatchInherited(t *testing.T) {
	pool := testPool(t)
	defer pool.Close()
	ctx := context.Background()

	perms := repository.NewPermissionRepository(pool)
	svc := authz.NewService(perms)
	users := repository.NewUserRepository(pool)
	objects := repository.NewMediaObjectRepository(pool)

	rootPID, _ := uuid.Parse("00000000-0000-4000-8000-000000000001")
	root, err := objects.GetByPublicID(ctx, rootPID)
	if err != nil {
		t.Fatalf("root folder: %v", err)
	}

	hash, err := auth.HashPassword("TestPass123")
	if err != nil {
		t.Fatal(err)
	}
	memberEmail := "authz-batch-" + uuid.New().String()[:8] + "@example.com"
	member, err := users.CreateMember(ctx, memberEmail, hash, "viewer", uuid.New(), 1)
	if err != nil {
		t.Fatalf("create member: %v", err)
	}
	defer func() {
		_, _ = pool.Exec(ctx, `DELETE FROM permissions WHERE user_id = $1`, member.ID)
		_, _ = pool.Exec(ctx, `DELETE FROM users WHERE id = $1`, member.ID)
	}()

	if _, err := perms.Grant(ctx, member.ID, root.ID, 1, "folder", string(authz.ActionRead)); err != nil {
		t.Fatalf("grant: %v", err)
	}
	if _, err := perms.Grant(ctx, member.ID, root.ID, 1, "folder", string(authz.ActionUpload)); err != nil {
		t.Fatalf("grant upload: %v", err)
	}

	childPID := uuid.New()
	var childID int64
	err = pool.QueryRow(ctx, `
		INSERT INTO media_objects (public_id, parent_id, type, name, status)
		VALUES ($1, $2, 'file', 'batch-child', 'active')
		RETURNING id
	`, childPID, root.ID).Scan(&childID)
	if err != nil {
		t.Fatalf("insert child: %v", err)
	}
	defer func() {
		_, _ = pool.Exec(ctx, `DELETE FROM object_paths WHERE descendant_id = $1`, childID)
		_, _ = pool.Exec(ctx, `DELETE FROM media_objects WHERE id = $1`, childID)
	}()

	_, err = pool.Exec(ctx, `
		INSERT INTO object_paths (ancestor_id, descendant_id, depth) VALUES ($1, $1, 0)
		ON CONFLICT DO NOTHING
	`, childID)
	if err != nil {
		t.Fatalf("self path: %v", err)
	}
	_, err = pool.Exec(ctx, `
		INSERT INTO object_paths (ancestor_id, descendant_id, depth) VALUES ($1, $2, 1)
		ON CONFLICT DO NOTHING
	`, root.ID, childID)
	if err != nil {
		t.Fatalf("ancestor path: %v", err)
	}

	capsMap, err := svc.CapabilitiesMap(ctx, member.ID, "viewer", []int64{childID})
	if err != nil {
		t.Fatalf("capabilities map: %v", err)
	}
	caps := capsMap[childID]
	if !caps.Read {
		t.Fatal("expected inherited read")
	}
	if !caps.Upload {
		t.Fatal("expected inherited upload from folder")
	}
	if caps.Delete {
		t.Fatal("did not grant delete")
	}
}

func TestCanListChildren_GrantedSubfolderWithoutParent(t *testing.T) {
	pool := testPool(t)
	defer pool.Close()
	ctx := context.Background()

	perms := repository.NewPermissionRepository(pool)
	svc := authz.NewService(perms)
	users := repository.NewUserRepository(pool)
	objects := repository.NewMediaObjectRepository(pool)

	rootPID, _ := uuid.Parse("00000000-0000-4000-8000-000000000001")
	root, err := objects.GetByPublicID(ctx, rootPID)
	if err != nil {
		t.Fatalf("root folder: %v", err)
	}

	hash, err := auth.HashPassword("TestPass123")
	if err != nil {
		t.Fatal(err)
	}
	memberEmail := "authz-subfolder-" + uuid.New().String()[:8] + "@example.com"
	member, err := users.CreateMember(ctx, memberEmail, hash, "viewer", uuid.New(), 1)
	if err != nil {
		t.Fatalf("create member: %v", err)
	}
	defer func() {
		_, _ = pool.Exec(ctx, `DELETE FROM permissions WHERE user_id = $1`, member.ID)
		_, _ = pool.Exec(ctx, `DELETE FROM users WHERE id = $1`, member.ID)
	}()

	midPID := uuid.New()
	mid, err := objects.CreateWithClosure(ctx, repository.CreateMediaObjectInput{
		PublicID:  midPID,
		ParentID:  &root.ID,
		Type:      "folder",
		Name:      "authz-mid-" + uuid.New().String()[:6],
		CreatedBy: 1,
	})
	if err != nil {
		t.Fatalf("create mid folder: %v", err)
	}
	defer func() {
		_ = objects.SoftDelete(ctx, mid.ID, 1)
	}()

	leafPID := uuid.New()
	leaf, err := objects.CreateWithClosure(ctx, repository.CreateMediaObjectInput{
		PublicID:  leafPID,
		ParentID:  &mid.ID,
		Type:      "folder",
		Name:      "authz-leaf-" + uuid.New().String()[:6],
		CreatedBy: 1,
	})
	if err != nil {
		t.Fatalf("create leaf folder: %v", err)
	}
	defer func() {
		_ = objects.SoftDelete(ctx, leaf.ID, 1)
	}()

	if _, err := perms.Grant(ctx, member.ID, leaf.ID, 1, "folder", string(authz.ActionRead)); err != nil {
		t.Fatalf("grant leaf: %v", err)
	}

	okRoot, err := svc.CanListChildren(ctx, member.ID, "viewer", root.ID)
	if err != nil || !okRoot {
		t.Fatalf("expected list access on root via shared descendant, got ok=%v err=%v", okRoot, err)
	}
	okMid, err := svc.CanListChildren(ctx, member.ID, "viewer", mid.ID)
	if err != nil || !okMid {
		t.Fatalf("expected list access on mid folder, got ok=%v err=%v", okMid, err)
	}
	okLeaf, err := svc.CanListChildren(ctx, member.ID, "viewer", leaf.ID)
	if err != nil || !okLeaf {
		t.Fatalf("expected list access on leaf folder, got ok=%v err=%v", okLeaf, err)
	}

	readRoot, err := svc.HasPermission(ctx, member.ID, "viewer", root.ID, authz.ActionRead)
	if err != nil || readRoot {
		t.Fatalf("root should not have direct/inherited read, got %v err=%v", readRoot, err)
	}

	siblingPID := uuid.New()
	sibling, err := objects.CreateWithClosure(ctx, repository.CreateMediaObjectInput{
		PublicID:  siblingPID,
		ParentID:  &root.ID,
		Type:      "folder",
		Name:      "authz-sibling-" + uuid.New().String()[:6],
		CreatedBy: 1,
	})
	if err != nil {
		t.Fatalf("create sibling folder: %v", err)
	}
	defer func() {
		_ = objects.SoftDelete(ctx, sibling.ID, 1)
	}()

	visible, err := svc.VisibleForListing(ctx, member.ID, "viewer", []int64{mid.ID, leaf.ID, sibling.ID})
	if err != nil {
		t.Fatalf("visible for listing: %v", err)
	}
	if _, ok := visible[mid.ID]; !ok {
		t.Fatal("expected mid folder on grant path")
	}
	if _, ok := visible[leaf.ID]; !ok {
		t.Fatal("expected leaf folder with direct grant")
	}
	if _, ok := visible[sibling.ID]; ok {
		t.Fatal("sibling folder without grant should be hidden")
	}
}

func TestCanListChildren_ManageGrantOnFolder(t *testing.T) {
	pool := testPool(t)
	defer pool.Close()
	ctx := context.Background()

	perms := repository.NewPermissionRepository(pool)
	svc := authz.NewService(perms)
	users := repository.NewUserRepository(pool)
	objects := repository.NewMediaObjectRepository(pool)

	rootPID, _ := uuid.Parse("00000000-0000-4000-8000-000000000001")
	root, err := objects.GetByPublicID(ctx, rootPID)
	if err != nil {
		t.Fatalf("root folder: %v", err)
	}

	hash, err := auth.HashPassword("TestPass123")
	if err != nil {
		t.Fatal(err)
	}
	memberEmail := "authz-manage-" + uuid.New().String()[:8] + "@example.com"
	member, err := users.CreateMember(ctx, memberEmail, hash, "viewer", uuid.New(), 1)
	if err != nil {
		t.Fatalf("create member: %v", err)
	}
	defer func() {
		_, _ = pool.Exec(ctx, `DELETE FROM permissions WHERE user_id = $1`, member.ID)
		_, _ = pool.Exec(ctx, `DELETE FROM users WHERE id = $1`, member.ID)
	}()

	leafPID := uuid.New()
	leaf, err := objects.CreateWithClosure(ctx, repository.CreateMediaObjectInput{
		PublicID:  leafPID,
		ParentID:  &root.ID,
		Type:      "folder",
		Name:      "authz-manage-leaf-" + uuid.New().String()[:6],
		CreatedBy: 1,
	})
	if err != nil {
		t.Fatalf("create leaf: %v", err)
	}
	defer func() {
		_ = objects.SoftDelete(ctx, leaf.ID, 1)
	}()

	if _, err := perms.Grant(ctx, member.ID, leaf.ID, 1, "folder", string(authz.ActionManage)); err != nil {
		t.Fatalf("grant manage: %v", err)
	}

	okRoot, err := svc.CanListChildren(ctx, member.ID, "viewer", root.ID)
	if err != nil || !okRoot {
		t.Fatalf("root list via manage grant below: ok=%v err=%v", okRoot, err)
	}
	okLeaf, err := svc.CanListChildren(ctx, member.ID, "viewer", leaf.ID)
	if err != nil || !okLeaf {
		t.Fatalf("open manage-granted folder: ok=%v err=%v", okLeaf, err)
	}
	readLeaf, err := svc.HasPermission(ctx, member.ID, "viewer", leaf.ID, authz.ActionRead)
	if err != nil || !readLeaf {
		t.Fatalf("manage should imply read on same folder: ok=%v err=%v", readLeaf, err)
	}
}
