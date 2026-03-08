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

func (h *TodoHandler) Hello(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{
		"message": "server is running",
	})
}

func (h *TodoHandler) Todos(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		todos := h.Service.GetAll()
		writeJSON(w, http.StatusOK, todos)

	case http.MethodPost:
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

		todo := h.Service.Create(req.Title)
		writeJSON(w, http.StatusCreated, todo)

	default:
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
	}
}

func (h *TodoHandler) TodoByID(w http.ResponseWriter, r *http.Request) {
	idStr := strings.TrimPrefix(r.URL.Path, "/todos/")
	id, err := strconv.Atoi(idStr)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid todo id")
		return
	}

	switch r.Method {
	case http.MethodGet:
		todo, err := h.Service.GetByID(id)
		if err != nil {
			writeError(w, http.StatusNotFound, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, todo)

	case http.MethodPut:
		var req UpdateTodoRequest

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

		todo, err := h.Service.Update(id, req.Title, req.Completed)
		if err != nil {
			writeError(w, http.StatusNotFound, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, todo)

	case http.MethodDelete:
		err := h.Service.Delete(id)
		if err != nil {
			writeError(w, http.StatusNotFound, err.Error())
			return
		}

		writeJSON(w, http.StatusOK, map[string]string{
			"message": "todo deleted",
		})

	default:
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
	}
}
