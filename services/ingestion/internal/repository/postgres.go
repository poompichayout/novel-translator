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
