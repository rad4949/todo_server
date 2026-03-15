package handlers

import (
	"encoding/json"
	"net/http"
	"strconv"
	"strings"
	"todo_server/services"
)

type TodoHandler struct {
	Service *services.TodoService
}

func NewTodoHandler(service *services.TodoService) *TodoHandler {
	return &TodoHandler{Service: service}
}

type CreateTodoRequest struct {
	Title string `json:"title"`
}

type UpdateTodoRequest struct {
	Title     string `json:"title"`
	Completed bool   `json:"completed"`
}

func writeJSON(w http.ResponseWriter, status int, data any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(data)
}

func writeError(w http.ResponseWriter, status int, message string) {
	writeJSON(w, status, map[string]string{
		"error": message,
	})
}

func getIDFromPath(path string) (int, error) {
	idStr := strings.TrimPrefix(path, "/todos/")
	return strconv.Atoi(idStr)
}

// Hello godoc
// @Summary Check server status
// @Description Returns server status message
// @Tags health
// @Produce json
// @Success 200 {object} map[string]string
// @Router / [get]
func (h *TodoHandler) Hello(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{
		"message": "server is running",
	})
}

// GetTodos godoc
// @Summary Get all todos
// @Description Get list of all todos
// @Tags todos
// @Produce json
// @Success 200 {array} models.Todo
// @Router /todos [get]
func (h *TodoHandler) GetTodos(w http.ResponseWriter, r *http.Request) {
	todos := h.Service.GetAll()
	writeJSON(w, http.StatusOK, todos)
}

// CreateTodo godoc
// @Summary Create new todo
// @Description Create a new todo item
// @Tags todos
// @Accept json
// @Produce json
// @Param todo body CreateTodoRequest true "Todo data"
// @Success 201 {object} models.Todo
// @Failure 400 {object} map[string]string
// @Failure 500 {object} map[string]string
// @Router /todos [post]
func (h *TodoHandler) CreateTodo(w http.ResponseWriter, r *http.Request) {
	var req CreateTodoRequest

	err := json.NewDecoder(r.Body).Decode(&req)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid json")
		return
	}

	req.Title = strings.TrimSpace(req.Title)
	if req.Title == "" {
		writeError(w, http.StatusBadRequest, "title is required")
		return
	}

	todo, err := h.Service.Create(req.Title)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	writeJSON(w, http.StatusCreated, todo)
}

// GetTodoByID godoc
// @Summary Get todo by ID
// @Description Get single todo item
// @Tags todos
// @Produce json
// @Param id path int true "Todo ID"
// @Success 200 {object} models.Todo
// @Failure 400 {object} map[string]string
// @Failure 404 {object} map[string]string
// @Router /todos/{id} [get]
func (h *TodoHandler) GetTodoByID(w http.ResponseWriter, r *http.Request) {
	id, err := getIDFromPath(r.URL.Path)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid todo id")
		return
	}

	todo, err := h.Service.GetByID(id)
	if err != nil {
		writeError(w, http.StatusNotFound, err.Error())
		return
	}

	writeJSON(w, http.StatusOK, todo)
}

// UpdateTodo godoc
// @Summary Update todo
// @Description Update todo title or status
// @Tags todos
// @Accept json
// @Produce json
// @Param id path int true "Todo ID"
// @Param todo body UpdateTodoRequest true "Updated todo data"
// @Success 200 {object} models.Todo
// @Failure 400 {object} map[string]string
// @Failure 404 {object} map[string]string
// @Router /todos/{id} [put]
func (h *TodoHandler) UpdateTodo(w http.ResponseWriter, r *http.Request) {
	id, err := getIDFromPath(r.URL.Path)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid todo id")
		return
	}

	var req UpdateTodoRequest

	err = json.NewDecoder(r.Body).Decode(&req)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid json")
		return
	}

	req.Title = strings.TrimSpace(req.Title)
	if req.Title == "" {
		writeError(w, http.StatusBadRequest, "title is required")
		return
	}

	todo, err := h.Service.Update(id, req.Title, req.Completed)
	if err != nil {
		writeError(w, http.StatusNotFound, err.Error())
		return
	}

	writeJSON(w, http.StatusOK, todo)
}

// DeleteTodo godoc
// @Summary Delete todo
// @Description Delete todo by ID
// @Tags todos
// @Produce json
// @Param id path int true "Todo ID"
// @Success 200 {object} map[string]string
// @Failure 400 {object} map[string]string
// @Failure 404 {object} map[string]string
// @Router /todos/{id} [delete]
func (h *TodoHandler) DeleteTodo(w http.ResponseWriter, r *http.Request) {
	id, err := getIDFromPath(r.URL.Path)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid todo id")
		return
	}

	err = h.Service.Delete(id)
	if err != nil {
		writeError(w, http.StatusNotFound, err.Error())
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{
		"message": "todo deleted",
	})
}
