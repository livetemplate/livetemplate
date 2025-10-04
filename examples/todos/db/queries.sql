-- name: GetAllTodos :many
SELECT * FROM todos
ORDER BY created_at DESC;

-- name: GetTodoByID :one
SELECT * FROM todos
WHERE id = ?
LIMIT 1;

-- name: CreateTodo :one
INSERT INTO todos (id, text, completed, created_at)
VALUES (?, ?, ?, ?)
RETURNING *;

-- name: UpdateTodoCompleted :exec
UPDATE todos
SET completed = ?
WHERE id = ?;

-- name: DeleteTodo :exec
DELETE FROM todos
WHERE id = ?;

-- name: DeleteCompletedTodos :exec
DELETE FROM todos
WHERE completed = 1;

-- name: CountTodos :one
SELECT COUNT(*) FROM todos;

-- name: CountCompletedTodos :one
SELECT COUNT(*) FROM todos
WHERE completed = 1;
