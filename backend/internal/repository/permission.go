package repository

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Permission struct {
	ID             int64
	UserID         int64
	UserPublicID   uuid.UUID
	UserEmail      string
	ResourceType   string
	ResourceID     int64
	ResourcePublic uuid.UUID
	ResourceName   string
	Permission     string
	GrantedBy      *int64
	ExpiresAt      *time.Time
	RevokedAt      *time.Time
	CreatedAt      time.Time
}

type PermissionRepository struct {
	pool *pgxpool.Pool
}

func NewPermissionRepository(pool *pgxpool.Pool) *PermissionRepository {
	return &PermissionRepository{pool: pool}
}

func (r *PermissionRepository) Grant(ctx context.Context, userID, resourceID, grantedBy int64, resourceType, permission string) (*Permission, error) {
	var p Permission
	err := r.pool.QueryRow(ctx, `
		INSERT INTO permissions (user_id, resource_type, resource_id, permission, granted_by)
		VALUES ($1, $2, $3, $4, $5)
		RETURNING id, user_id, resource_type, resource_id, permission, granted_by, expires_at, revoked_at, created_at
	`, userID, resourceType, resourceID, permission, grantedBy).Scan(
		&p.ID, &p.UserID, &p.ResourceType, &p.ResourceID, &p.Permission, &p.GrantedBy, &p.ExpiresAt, &p.RevokedAt, &p.CreatedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("grant permission: %w", err)
	}
	return &p, nil
}

func (r *PermissionRepository) Revoke(ctx context.Context, id int64) error {
	_, err := r.pool.Exec(ctx, `
		UPDATE permissions SET revoked_at = now() WHERE id = $1 AND revoked_at IS NULL
	`, id)
	if err != nil {
		return fmt.Errorf("revoke permission: %w", err)
	}
	return nil
}

func (r *PermissionRepository) ListByResource(ctx context.Context, resourceID int64) ([]Permission, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT p.id, p.user_id, u.public_id, u.email, p.resource_type, p.resource_id,
		       m.public_id, m.name, p.permission, p.granted_by, p.expires_at, p.revoked_at, p.created_at
		FROM permissions p
		JOIN users u ON u.id = p.user_id
		JOIN media_objects m ON m.id = p.resource_id
		WHERE p.resource_id = $1 AND p.revoked_at IS NULL
		ORDER BY p.created_at DESC
	`, resourceID)
	if err != nil {
		return nil, fmt.Errorf("list permissions: %w", err)
	}
	defer rows.Close()

	var list []Permission
	for rows.Next() {
		var p Permission
		if err := rows.Scan(
			&p.ID, &p.UserID, &p.UserPublicID, &p.UserEmail, &p.ResourceType, &p.ResourceID,
			&p.ResourcePublic, &p.ResourceName, &p.Permission, &p.GrantedBy, &p.ExpiresAt, &p.RevokedAt, &p.CreatedAt,
		); err != nil {
			return nil, err
		}
		list = append(list, p)
	}
	return list, rows.Err()
}

func (r *PermissionRepository) HasDirect(ctx context.Context, userID, resourceID int64, permission string) (bool, error) {
	var exists bool
	err := r.pool.QueryRow(ctx, `
		SELECT EXISTS(
			SELECT 1 FROM permissions
			WHERE user_id = $1 AND resource_id = $2 AND permission = $3
			  AND revoked_at IS NULL
			  AND (expires_at IS NULL OR expires_at > now())
		)
	`, userID, resourceID, permission).Scan(&exists)
	return exists, err
}

// HasDirectAny is true when the user has any active permission grant on the resource.
func (r *PermissionRepository) HasDirectAny(ctx context.Context, userID, resourceID int64) (bool, error) {
	var exists bool
	err := r.pool.QueryRow(ctx, `
		SELECT EXISTS(
			SELECT 1 FROM permissions
			WHERE user_id = $1 AND resource_id = $2
			  AND revoked_at IS NULL
			  AND (expires_at IS NULL OR expires_at > now())
		)
	`, userID, resourceID).Scan(&exists)
	return exists, err
}

// HasGrantedDescendant is true when the user has a direct grant on any descendant (folder or file).
func (r *PermissionRepository) HasGrantedDescendant(ctx context.Context, userID, ancestorID int64, permission string) (bool, error) {
	perms := []string{permission}
	if permission == "read" {
		perms = []string{"read", "manage"}
	}
	var exists bool
	err := r.pool.QueryRow(ctx, `
		WITH RECURSIVE desc_tree AS (
			SELECT id FROM media_objects WHERE id = $1 AND deleted_at IS NULL
			UNION ALL
			SELECT m.id
			FROM media_objects m
			INNER JOIN desc_tree d ON m.parent_id = d.id
			WHERE m.deleted_at IS NULL
		)
		SELECT EXISTS(
			SELECT 1
			FROM desc_tree d
			JOIN permissions p ON p.resource_id = d.id
			WHERE d.id != $1
			  AND p.user_id = $2
			  AND p.permission = ANY($3)
			  AND p.revoked_at IS NULL
			  AND (p.expires_at IS NULL OR p.expires_at > now())
		)
	`, ancestorID, userID, perms).Scan(&exists)
	return exists, err
}

// HasGrantedDescendantAny is true when the user has any direct grant on a descendant.
func (r *PermissionRepository) HasGrantedDescendantAny(ctx context.Context, userID, ancestorID int64) (bool, error) {
	var exists bool
	err := r.pool.QueryRow(ctx, `
		WITH RECURSIVE desc_tree AS (
			SELECT id FROM media_objects WHERE id = $1 AND deleted_at IS NULL
			UNION ALL
			SELECT m.id
			FROM media_objects m
			INNER JOIN desc_tree d ON m.parent_id = d.id
			WHERE m.deleted_at IS NULL
		)
		SELECT EXISTS(
			SELECT 1
			FROM desc_tree d
			JOIN permissions p ON p.resource_id = d.id
			WHERE d.id != $1
			  AND p.user_id = $2
			  AND p.revoked_at IS NULL
			  AND (p.expires_at IS NULL OR p.expires_at > now())
		)
	`, ancestorID, userID).Scan(&exists)
	return exists, err
}

// AncestorIDsWithGrantedDescendant returns ancestor IDs (from the input set) that have a
// direct permission grant somewhere in their subtree (excluding self).
func (r *PermissionRepository) AncestorIDsWithGrantedDescendant(
	ctx context.Context,
	userID int64,
	ancestorIDs []int64,
	permission string,
) (map[int64]struct{}, error) {
	out := make(map[int64]struct{})
	if len(ancestorIDs) == 0 {
		return out, nil
	}
	rows, err := r.pool.Query(ctx, `
		SELECT DISTINCT op.ancestor_id
		FROM object_paths op
		JOIN permissions p ON p.resource_id = op.descendant_id
		WHERE op.ancestor_id = ANY($1)
		  AND op.depth > 0
		  AND p.user_id = $2
		  AND p.permission = $3
		  AND p.revoked_at IS NULL
		  AND (p.expires_at IS NULL OR p.expires_at > now())
	`, ancestorIDs, userID, permission)
	if err != nil {
		return nil, fmt.Errorf("ancestors with granted descendant: %w", err)
	}
	defer rows.Close()
	for rows.Next() {
		var id int64
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		out[id] = struct{}{}
	}
	return out, rows.Err()
}

// ListingVisibleAmong returns object IDs that may appear in a folder listing for a member:
// direct read grant, read inherited from a directly-granted folder ancestor, or a folder on the path
// to a directly-granted resource (any permission counts for path discovery).
func (r *PermissionRepository) ListingVisibleAmong(
	ctx context.Context,
	userID int64,
	resourceIDs []int64,
) (map[int64]struct{}, error) {
	out := make(map[int64]struct{})
	if len(resourceIDs) == 0 {
		return out, nil
	}
	rows, err := r.pool.Query(ctx, `
		WITH direct_read AS (
			SELECT p.resource_id AS id
			FROM permissions p
			WHERE p.user_id = $1
			  AND p.permission = 'read'
			  AND p.resource_id = ANY($2)
			  AND p.revoked_at IS NULL
			  AND (p.expires_at IS NULL OR p.expires_at > now())
		),
		inherited_read AS (
			SELECT DISTINCT m.id
			FROM media_objects m
			WHERE m.id = ANY($2)
			  AND EXISTS (
			    WITH RECURSIVE anc_tree AS (
			      SELECT id, parent_id FROM media_objects WHERE id = m.id AND deleted_at IS NULL
			      UNION ALL
			      SELECT p.id, p.parent_id
			      FROM media_objects p
			      INNER JOIN anc_tree a ON p.id = a.parent_id
			      WHERE p.deleted_at IS NULL
			    )
			    SELECT 1
			    FROM anc_tree a
			    JOIN permissions p ON p.resource_id = a.id
			    WHERE a.id != m.id
			      AND p.user_id = $1
			      AND p.permission IN ('read', 'manage')
			      AND p.resource_type = 'folder'
			      AND p.revoked_at IS NULL
			      AND (p.expires_at IS NULL OR p.expires_at > now())
			  )
		),
		direct_any AS (
			SELECT p.resource_id AS id
			FROM permissions p
			WHERE p.user_id = $1
			  AND p.resource_id = ANY($2)
			  AND p.revoked_at IS NULL
			  AND (p.expires_at IS NULL OR p.expires_at > now())
		),
		path_folder AS (
			SELECT DISTINCT m.id
			FROM media_objects m
			WHERE m.id = ANY($2)
			  AND m.type = 'folder'
			  AND EXISTS (
			    WITH RECURSIVE desc_tree AS (
			      SELECT id FROM media_objects WHERE id = m.id AND deleted_at IS NULL
			      UNION ALL
			      SELECT c.id
			      FROM media_objects c
			      INNER JOIN desc_tree d ON c.parent_id = d.id
			      WHERE c.deleted_at IS NULL
			    )
			    SELECT 1
			    FROM desc_tree d
			    JOIN permissions p ON p.resource_id = d.id
			    WHERE d.id != m.id
			      AND p.user_id = $1
			      AND p.revoked_at IS NULL
			      AND (p.expires_at IS NULL OR p.expires_at > now())
			  )
		)
		SELECT id FROM direct_read
		UNION SELECT id FROM inherited_read
		UNION SELECT id FROM direct_any
		UNION SELECT id FROM path_folder
	`, userID, resourceIDs)
	if err != nil {
		return nil, fmt.Errorf("listing visible among: %w", err)
	}
	defer rows.Close()
	for rows.Next() {
		var id int64
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		out[id] = struct{}{}
	}
	return out, rows.Err()
}

func (r *PermissionRepository) HasInherited(ctx context.Context, userID, resourceID int64, permission string) (bool, error) {
	perms := []string{permission}
	if permission == "read" {
		perms = []string{"read", "manage"}
	}
	var exists bool
	err := r.pool.QueryRow(ctx, `
		WITH RECURSIVE anc_tree AS (
			SELECT id, parent_id FROM media_objects WHERE id = $2 AND deleted_at IS NULL
			UNION ALL
			SELECT m.id, m.parent_id
			FROM media_objects m
			INNER JOIN anc_tree a ON m.id = a.parent_id
			WHERE m.deleted_at IS NULL
		)
		SELECT EXISTS(
			SELECT 1
			FROM anc_tree a
			JOIN permissions p ON p.resource_id = a.id
			JOIN media_objects m ON m.id = a.id AND m.type = 'folder'
			WHERE a.id != $2
			  AND p.user_id = $1
			  AND p.permission = ANY($3)
			  AND p.resource_type = 'folder'
			  AND p.revoked_at IS NULL
			  AND (p.expires_at IS NULL OR p.expires_at > now())
		)
	`, userID, resourceID, perms).Scan(&exists)
	return exists, err
}

// EffectivePermissionsForResources returns merged direct + inherited permission names per resource ID.
func (r *PermissionRepository) EffectivePermissionsForResources(
	ctx context.Context,
	userID int64,
	resourceIDs []int64,
) (map[int64]map[string]struct{}, error) {
	out := make(map[int64]map[string]struct{}, len(resourceIDs))
	if len(resourceIDs) == 0 {
		return out, nil
	}
	for _, id := range resourceIDs {
		out[id] = make(map[string]struct{})
	}

	rows, err := r.pool.Query(ctx, `
		SELECT resource_id, permission
		FROM permissions
		WHERE user_id = $1
		  AND resource_id = ANY($2)
		  AND revoked_at IS NULL
		  AND (expires_at IS NULL OR expires_at > now())
	`, userID, resourceIDs)
	if err != nil {
		return nil, fmt.Errorf("list direct permissions: %w", err)
	}
	for rows.Next() {
		var resourceID int64
		var perm string
		if err := rows.Scan(&resourceID, &perm); err != nil {
			rows.Close()
			return nil, err
		}
		out[resourceID][perm] = struct{}{}
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return nil, err
	}
	rows.Close()

	rows, err = r.pool.Query(ctx, `
		SELECT op.descendant_id, p.permission
		FROM object_paths op
		JOIN permissions p ON p.resource_id = op.ancestor_id
		WHERE op.descendant_id = ANY($1)
		  AND op.depth > 0
		  AND p.user_id = $2
		  AND p.resource_type = 'folder'
		  AND p.revoked_at IS NULL
		  AND (p.expires_at IS NULL OR p.expires_at > now())
	`, resourceIDs, userID)
	if err != nil {
		return nil, fmt.Errorf("list inherited permissions: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		var resourceID int64
		var perm string
		if err := rows.Scan(&resourceID, &perm); err != nil {
			return nil, err
		}
		if out[resourceID] == nil {
			out[resourceID] = make(map[string]struct{})
		}
		out[resourceID][perm] = struct{}{}
	}
	return out, rows.Err()
}

func (r *PermissionRepository) ListByUser(ctx context.Context, userID int64) ([]Permission, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT p.id, p.user_id, u.public_id, u.email, p.resource_type, p.resource_id,
		       m.public_id, m.name, p.permission, p.granted_by, p.expires_at, p.revoked_at, p.created_at
		FROM permissions p
		JOIN users u ON u.id = p.user_id
		JOIN media_objects m ON m.id = p.resource_id
		WHERE p.user_id = $1 AND p.revoked_at IS NULL
		ORDER BY p.created_at DESC
	`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var list []Permission
	for rows.Next() {
		var p Permission
		if err := rows.Scan(
			&p.ID, &p.UserID, &p.UserPublicID, &p.UserEmail, &p.ResourceType, &p.ResourceID,
			&p.ResourcePublic, &p.ResourceName, &p.Permission, &p.GrantedBy, &p.ExpiresAt, &p.RevokedAt, &p.CreatedAt,
		); err != nil {
			return nil, err
		}
		list = append(list, p)
	}
	return list, rows.Err()
}
