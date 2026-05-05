package repository

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/poompich/novel-translator/services/ingestion/internal/domain"
)

type PostgresRepo struct {
	db *pgxpool.Pool
}

func NewPostgresRepo(ctx context.Context, connString string) (*PostgresRepo, error) {
	pool, err := pgxpool.New(ctx, connString)
	if err != nil {
		return nil, fmt.Errorf("unable to connect to database: %w", err)
	}

	if err := pool.Ping(ctx); err != nil {
		return nil, fmt.Errorf("database ping failed: %w", err)
	}

	return &PostgresRepo{db: pool}, nil
}

func (r *PostgresRepo) Close() {
	if r.db != nil {
		r.db.Close()
	}
}

func (r *PostgresRepo) UpsertNovel(ctx context.Context, novel domain.Novel) (int, error) {
	query := `
		INSERT INTO novels (title, source_url, source_lang, target_lang, status)
		VALUES ($1, $2, $3, $4, $5)
		ON CONFLICT (source_url) DO UPDATE SET
			title = EXCLUDED.title,
			status = EXCLUDED.status
		RETURNING id
	`
	var id int
	err := r.db.QueryRow(ctx, query, novel.Title, novel.SourceURL, novel.SourceLang, novel.TargetLang, novel.Status).Scan(&id)
	if err != nil {
		return 0, fmt.Errorf("failed to upsert novel: %w", err)
	}
	return id, nil
}

func (r *PostgresRepo) UpsertChapter(ctx context.Context, chapter domain.Chapter) (int, error) {
	query := `
		INSERT INTO chapters (novel_id, chapter_number, title, raw_content, cleaned_content, source_url, scrape_status)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
		ON CONFLICT (novel_id, chapter_number) DO UPDATE SET
			title = EXCLUDED.title,
			raw_content = EXCLUDED.raw_content,
			cleaned_content = EXCLUDED.cleaned_content,
			source_url = EXCLUDED.source_url,
			scrape_status = EXCLUDED.scrape_status
		RETURNING id
	`
	var id int
	err := r.db.QueryRow(ctx, query,
		chapter.NovelID, chapter.ChapterNumber, chapter.Title,
		chapter.RawContent, chapter.CleanedContent, chapter.SourceURL, chapter.ScrapeStatus,
	).Scan(&id)

	if err != nil {
		return 0, fmt.Errorf("failed to upsert chapter %d: %w", chapter.ChapterNumber, err)
	}
	return id, nil
}

func (r *PostgresRepo) GetNovel(ctx context.Context, id int) (domain.Novel, error) {
	var n domain.Novel
	err := r.db.QueryRow(ctx, `
		SELECT id, title, source_url, source_lang, target_lang, status, created_at, updated_at
		FROM novels WHERE id = $1
	`, id).Scan(&n.ID, &n.Title, &n.SourceURL, &n.SourceLang, &n.TargetLang, &n.Status, &n.CreatedAt, &n.UpdatedAt)
	if err != nil {
		return domain.Novel{}, fmt.Errorf("get novel %d: %w", id, err)
	}
	return n, nil
}

func (r *PostgresRepo) ListNovels(ctx context.Context) ([]domain.Novel, error) {
	rows, err := r.db.Query(ctx, `
		SELECT id, title, source_url, source_lang, target_lang, status, created_at, updated_at
		FROM novels ORDER BY id DESC
	`)
	if err != nil {
		return nil, fmt.Errorf("list novels: %w", err)
	}
	defer rows.Close()
	var out []domain.Novel
	for rows.Next() {
		var n domain.Novel
		if err := rows.Scan(&n.ID, &n.Title, &n.SourceURL, &n.SourceLang, &n.TargetLang, &n.Status, &n.CreatedAt, &n.UpdatedAt); err != nil {
			return nil, fmt.Errorf("scan novel: %w", err)
		}
		out = append(out, n)
	}
	return out, rows.Err()
}

func (r *PostgresRepo) GetNextChapterNumber(ctx context.Context, novelID int) (int, error) {
	var maxN *int
	err := r.db.QueryRow(ctx, `SELECT MAX(chapter_number) FROM chapters WHERE novel_id = $1`, novelID).Scan(&maxN)
	if err != nil {
		return 0, fmt.Errorf("max chapter for novel %d: %w", novelID, err)
	}
	if maxN == nil {
		return 1, nil
	}
	return *maxN + 1, nil
}

func (r *PostgresRepo) ListChapters(ctx context.Context, novelID int) ([]domain.Chapter, error) {
	rows, err := r.db.Query(ctx, `
		SELECT id, novel_id, chapter_number, title, source_url, scrape_status, created_at, updated_at
		FROM chapters WHERE novel_id = $1 ORDER BY chapter_number ASC
	`, novelID)
	if err != nil {
		return nil, fmt.Errorf("list chapters: %w", err)
	}
	defer rows.Close()
	var out []domain.Chapter
	for rows.Next() {
		var c domain.Chapter
		var title, srcURL *string
		if err := rows.Scan(&c.ID, &c.NovelID, &c.ChapterNumber, &title, &srcURL, &c.ScrapeStatus, &c.CreatedAt, &c.UpdatedAt); err != nil {
			return nil, fmt.Errorf("scan chapter: %w", err)
		}
		if title != nil {
			c.Title = *title
		}
		if srcURL != nil {
			c.SourceURL = *srcURL
		}
		out = append(out, c)
	}
	return out, rows.Err()
}

func (r *PostgresRepo) UpsertEntity(ctx context.Context, entity domain.Entity) (int, error) {
	query := `
		INSERT INTO entities (novel_id, name_en, name_th, type, aliases, description)
		VALUES ($1, $2, $3, $4, $5, $6)
		ON CONFLICT (novel_id, name_en) DO UPDATE SET
			name_th = EXCLUDED.name_th,
			type = EXCLUDED.type,
			aliases = EXCLUDED.aliases,
			description = EXCLUDED.description
		RETURNING id
	`
	var id int
	err := r.db.QueryRow(ctx, query,
		entity.NovelID, entity.NameEn, entity.NameTh, entity.Type, entity.Aliases, entity.Description,
	).Scan(&id)

	if err != nil {
		return 0, fmt.Errorf("failed to upsert entity '%s': %w", entity.NameEn, err)
	}
	return id, nil
}
