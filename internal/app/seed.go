package app

import (
	"context"
	"encoding/csv"
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// SeedTasksIfEmpty загружает задачи из data/tasks_seed.csv, пропуская уже существующие (по title и points).
// Возвращает количество импортированных задач и ошибку.
func SeedTasksIfEmpty(ctx context.Context, pool *pgxpool.Pool) (imported int, err error) {
	// Открываем CSV-файл
	const path = "data/tasks_seed.csv"
	f, err := os.Open(path)
	if err != nil {
		return 0, fmt.Errorf("open csv: %w", err)
	}
	defer f.Close()

	reader := csv.NewReader(f)
	reader.TrimLeadingSpace = true

	records, err := reader.ReadAll()
	if err != nil {
		return 0, fmt.Errorf("read csv: %w", err)
	}
	if len(records) == 0 {
		return 0, nil
	}

	// Ожидаем заголовок: title,description,points,devs,analysts,devops,designers,testers,repeatable
	for i, row := range records {
		if i == 0 {
			continue
		}
		if len(row) < 9 {
			continue
		}
		title := strings.TrimSpace(row[0])
		desc := strings.TrimSpace(row[1])
		if title == "" {
			continue
		}
		points, err := strconv.Atoi(strings.TrimSpace(row[2]))
		if err != nil {
			continue
		}
		devs, _ := strconv.Atoi(strings.TrimSpace(row[3]))
		analysts, _ := strconv.Atoi(strings.TrimSpace(row[4]))
		devops, _ := strconv.Atoi(strings.TrimSpace(row[5]))
		designers, _ := strconv.Atoi(strings.TrimSpace(row[6]))
		testers, _ := strconv.Atoi(strings.TrimSpace(row[7]))
		repeatableStr := strings.TrimSpace(row[8])
		repeatable := repeatableStr == "true"

		// Проверяем, существует ли уже задача с таким title и points
		var existingID int64
		err = pool.QueryRow(ctx, `SELECT id FROM tasks WHERE title=$1 AND points=$2 LIMIT 1`, title, points).Scan(&existingID)
		if err != nil && err != pgx.ErrNoRows {
			continue
		}
		if existingID != 0 {
			continue
		}

		// Вставляем задачу
		_, err = pool.Exec(ctx, `
INSERT INTO tasks(title, description, points, devs, analysts, devops, designers, testers, repeatable, active)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, true)`,
			title, desc, points,
			clampResource(devs), clampResource(analysts), clampResource(devops), clampResource(designers), clampResource(testers),
			repeatable)
		if err != nil {
			// Пропускаем ошибку, продолжаем
			continue
		}
		imported++
	}

	return imported, nil
}
