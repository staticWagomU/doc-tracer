package db

import (
	"database/sql"
	"time"

	_ "modernc.org/sqlite"
)

// Node はコード要素またはドキュメントを表す
type Node struct {
	ID        string    `json:"id"`
	Type      string    `json:"type"`       // document, function, component, api, db_column, store, composable
	Name      string    `json:"name"`
	FilePath  string    `json:"file_path"`
	FileHash  string    `json:"file_hash"`  // 変更検知用
	LineStart int       `json:"line_start"` // コード要素の場合
	LineEnd   int       `json:"line_end"`
	Metadata  string    `json:"metadata"`   // JSON
	UpdatedAt time.Time `json:"updated_at"`
}

// Edge はノード間の関係を表す
type Edge struct {
	ID        int       `json:"id"`
	FromID    string    `json:"from_id"`
	ToID      string    `json:"to_id"`
	Relation  string    `json:"relation"`   // implements, calls, documents, uses, displays, stores
	Source    string    `json:"source"`     // auto or manual
	DefinedIn string    `json:"defined_in"` // どのファイルで定義されたか
	CreatedAt time.Time `json:"created_at"`
}

// DB はSQLiteデータベースへの接続を管理
type DB struct {
	conn *sql.DB
}

// Open はデータベースを開く
func Open(path string) (*DB, error) {
	conn, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, err
	}

	db := &DB{conn: conn}
	if err := db.migrate(); err != nil {
		conn.Close()
		return nil, err
	}

	return db, nil
}

// Close はデータベースを閉じる
func (db *DB) Close() error {
	return db.conn.Close()
}

// migrate はスキーマを作成/更新する
func (db *DB) migrate() error {
	schema := `
	CREATE TABLE IF NOT EXISTS nodes (
		id TEXT PRIMARY KEY,
		type TEXT NOT NULL,
		name TEXT NOT NULL,
		file_path TEXT,
		file_hash TEXT,
		line_start INTEGER,
		line_end INTEGER,
		metadata TEXT,
		updated_at DATETIME DEFAULT CURRENT_TIMESTAMP,
		UNIQUE(file_path, name, type)
	);

	CREATE TABLE IF NOT EXISTS edges (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		from_id TEXT NOT NULL,
		to_id TEXT NOT NULL,
		relation TEXT NOT NULL,
		source TEXT DEFAULT 'auto',
		defined_in TEXT,
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
		UNIQUE(from_id, to_id, relation),
		FOREIGN KEY (from_id) REFERENCES nodes(id),
		FOREIGN KEY (to_id) REFERENCES nodes(id)
	);

	CREATE TABLE IF NOT EXISTS scan_history (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		file_path TEXT NOT NULL,
		file_hash TEXT NOT NULL,
		scanned_at DATETIME DEFAULT CURRENT_TIMESTAMP
	);

	CREATE INDEX IF NOT EXISTS idx_nodes_type ON nodes(type);
	CREATE INDEX IF NOT EXISTS idx_nodes_file_path ON nodes(file_path);
	CREATE INDEX IF NOT EXISTS idx_edges_from_id ON edges(from_id);
	CREATE INDEX IF NOT EXISTS idx_edges_to_id ON edges(to_id);
	`

	_, err := db.conn.Exec(schema)
	return err
}

// UpsertNode はノードを挿入または更新する
func (db *DB) UpsertNode(node *Node) error {
	_, err := db.conn.Exec(`
		INSERT INTO nodes (id, type, name, file_path, file_hash, line_start, line_end, metadata, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, CURRENT_TIMESTAMP)
		ON CONFLICT(id) DO UPDATE SET
			type = excluded.type,
			name = excluded.name,
			file_path = excluded.file_path,
			file_hash = excluded.file_hash,
			line_start = excluded.line_start,
			line_end = excluded.line_end,
			metadata = excluded.metadata,
			updated_at = CURRENT_TIMESTAMP
	`, node.ID, node.Type, node.Name, node.FilePath, node.FileHash, node.LineStart, node.LineEnd, node.Metadata)
	return err
}

// UpsertEdge はエッジを挿入または更新する
func (db *DB) UpsertEdge(edge *Edge) error {
	_, err := db.conn.Exec(`
		INSERT INTO edges (from_id, to_id, relation, source, defined_in, created_at)
		VALUES (?, ?, ?, ?, ?, CURRENT_TIMESTAMP)
		ON CONFLICT(from_id, to_id, relation) DO UPDATE SET
			source = excluded.source,
			defined_in = excluded.defined_in
	`, edge.FromID, edge.ToID, edge.Relation, edge.Source, edge.DefinedIn)
	return err
}

// GetImpactedDocs は指定ファイルから影響を受けるドキュメントを取得（上方向にトレース）
func (db *DB) GetImpactedDocs(filePath string) ([]Node, error) {
	// 再帰CTEで上方向に辿る
	query := `
	WITH RECURSIVE impact_chain AS (
		-- 起点: 指定ファイルに含まれるノード
		SELECT n.id, n.type, n.name, n.file_path, 0 as depth
		FROM nodes n
		WHERE n.file_path = ?

		UNION

		-- 再帰: このノードを参照しているノードを辿る
		SELECT n.id, n.type, n.name, n.file_path, ic.depth + 1
		FROM nodes n
		JOIN edges e ON e.from_id = n.id
		JOIN impact_chain ic ON e.to_id = ic.id
		WHERE ic.depth < 10
	)
	SELECT DISTINCT id, type, name, file_path
	FROM impact_chain
	WHERE type = 'document'
	ORDER BY depth
	`

	rows, err := db.conn.Query(query, filePath)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var docs []Node
	for rows.Next() {
		var node Node
		if err := rows.Scan(&node.ID, &node.Type, &node.Name, &node.FilePath); err != nil {
			return nil, err
		}
		docs = append(docs, node)
	}

	return docs, nil
}

// GetAllNodes は全ノードを取得
func (db *DB) GetAllNodes() ([]Node, error) {
	rows, err := db.conn.Query(`SELECT id, type, name, file_path, file_hash, line_start, line_end, metadata, updated_at FROM nodes`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var nodes []Node
	for rows.Next() {
		var node Node
		if err := rows.Scan(&node.ID, &node.Type, &node.Name, &node.FilePath, &node.FileHash, &node.LineStart, &node.LineEnd, &node.Metadata, &node.UpdatedAt); err != nil {
			return nil, err
		}
		nodes = append(nodes, node)
	}

	return nodes, nil
}

// GetAllEdges は全エッジを取得
func (db *DB) GetAllEdges() ([]Edge, error) {
	rows, err := db.conn.Query(`SELECT id, from_id, to_id, relation, source, defined_in, created_at FROM edges`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var edges []Edge
	for rows.Next() {
		var edge Edge
		if err := rows.Scan(&edge.ID, &edge.FromID, &edge.ToID, &edge.Relation, &edge.Source, &edge.DefinedIn, &edge.CreatedAt); err != nil {
			return nil, err
		}
		edges = append(edges, edge)
	}

	return edges, nil
}

// CheckBrokenLinks はリンク切れを検出
func (db *DB) CheckBrokenLinks() ([]Edge, error) {
	query := `
	SELECT e.id, e.from_id, e.to_id, e.relation, e.source, e.defined_in, e.created_at
	FROM edges e
	LEFT JOIN nodes n1 ON e.from_id = n1.id
	LEFT JOIN nodes n2 ON e.to_id = n2.id
	WHERE n1.id IS NULL OR n2.id IS NULL
	`

	rows, err := db.conn.Query(query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var broken []Edge
	for rows.Next() {
		var edge Edge
		if err := rows.Scan(&edge.ID, &edge.FromID, &edge.ToID, &edge.Relation, &edge.Source, &edge.DefinedIn, &edge.CreatedAt); err != nil {
			return nil, err
		}
		broken = append(broken, edge)
	}

	return broken, nil
}

// GetOrphanedDocs は孤立ドキュメント（参照されてない）を検出
func (db *DB) GetOrphanedDocs() ([]Node, error) {
	query := `
	SELECT n.id, n.type, n.name, n.file_path, n.file_hash, n.line_start, n.line_end, n.metadata, n.updated_at
	FROM nodes n
	LEFT JOIN edges e ON n.id = e.from_id OR n.id = e.to_id
	WHERE n.type = 'document' AND e.id IS NULL
	`

	rows, err := db.conn.Query(query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var orphans []Node
	for rows.Next() {
		var node Node
		if err := rows.Scan(&node.ID, &node.Type, &node.Name, &node.FilePath, &node.FileHash, &node.LineStart, &node.LineEnd, &node.Metadata, &node.UpdatedAt); err != nil {
			return nil, err
		}
		orphans = append(orphans, node)
	}

	return orphans, nil
}
