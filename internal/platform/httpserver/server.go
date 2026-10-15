package httpserver

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"log/slog"
	"mime"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/skif/flashka/internal/auth"
	"github.com/skif/flashka/internal/cards"
	"github.com/skif/flashka/internal/folders"
	"github.com/skif/flashka/internal/platform/config"
	"github.com/skif/flashka/internal/study"
)

type ctxKey string

const requestIDKey ctxKey = "request-id"

type requestMeta struct {
	id     string
	userID string
}

type Server struct {
	cfg     config.Config
	pool    *pgxpool.Pool
	auth    auth.Service
	cards   cards.Service
	folders folders.Service
	study   study.Service
	limiter *auth.LoginLimiter
	log     *slog.Logger
}

func New(cfg config.Config, pool *pgxpool.Pool, service auth.Service, log *slog.Logger) http.Handler {
	s := &Server{cfg: cfg, pool: pool, auth: service, cards: cards.Service{Repo: cards.Repository{Pool: pool}, Clock: service.Clock}, folders: folders.Service{Repo: folders.Repository{Pool: pool}, Clock: service.Clock}, study: study.Service{Repo: study.Repository{Pool: pool}, Clock: service.Clock}, limiter: auth.NewLoginLimiter(service.Clock), log: log}
	r := chi.NewRouter()
	r.Use(s.requestContext, s.accessLog, s.recoverer, s.security)
	r.Get("/health/live", s.live)
	r.Get("/health/ready", s.ready)
	r.Route("/api/v1", func(r chi.Router) {
		r.Use(noStore)
		r.Route("/auth", func(r chi.Router) {
			r.With(s.csrf).Post("/register", s.register)
			r.With(s.csrf).Post("/login", s.login)
			r.With(s.csrf).Post("/logout", s.logout)
			r.Get("/me", s.me)
		})
		r.Route("/cards", func(r chi.Router) {
			r.Use(s.authenticated)
			r.Get("/", s.listCards)
			r.With(s.csrf).Post("/", s.createCard)
			r.Route("/{cardId}", func(r chi.Router) {
				r.Get("/", s.getCard)
				r.With(s.csrf).Patch("/", s.updateCard)
				r.With(s.csrf).Delete("/", s.deleteCard)
			})
		})
		r.Route("/folders", func(r chi.Router) {
			r.Use(s.authenticated)
			r.Get("/tree", s.folderTree)
			r.With(s.csrf).Post("/", s.createFolder)
			r.Route("/{folderId}", func(r chi.Router) {
				r.With(s.csrf).Patch("/", s.updateFolder)
				r.With(s.csrf).Delete("/", s.deleteFolder)
			})
		})
		r.Route("/study", func(r chi.Router) {
			r.Use(s.authenticated)
			r.Get("/preview", s.studyPreview)
			r.With(s.csrf).Post("/sessions", s.createStudySession)
			r.Get("/sessions/active", s.activeStudySession)
			r.Route("/sessions/{sessionId}", func(r chi.Router) {
				r.Get("/", s.getStudySession)
				r.Get("/result", s.studySessionResult)
				r.With(s.csrf).Post("/cards/{position}/reveal", s.revealStudyPosition)
				r.With(s.csrf).Put("/cards/{position}/answer", s.answerStudyPosition)
				r.With(s.csrf).Post("/abandon", s.abandonStudySession)
			})
		})
	})
	r.Handle("/*", s.frontend())
	return r
}

type createStudySessionInput struct {
	ScopeType string  `json:"scopeType"`
	FolderID  *string `json:"folderId"`
	Count     int     `json:"count"`
}

func studyScope(w http.ResponseWriter, r *http.Request, rawFolder *string) (*uuid.UUID, bool) {
	folderID, ok := optionalUUID(w, r, rawFolder, "folderId")
	if !ok {
		return nil, false
	}
	return folderID, true
}

func (s *Server) studyPreview(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	var rawFolder *string
	if q.Has("folderId") {
		value := q.Get("folderId")
		rawFolder = &value
	}
	folderID, ok := studyScope(w, r, rawFolder)
	if !ok {
		return
	}
	preview, fields, err := s.study.Preview(r.Context(), currentUserID(r), q.Get("scopeType"), folderID)
	if fields != nil {
		writeError(w, r, 422, "validation_failed", "Проверьте параметры", fields)
		return
	}
	if errors.Is(err, study.ErrFolderNotFound) {
		writeError(w, r, 404, "folder_not_found", "Папка не найдена", nil)
		return
	}
	if err != nil {
		s.internal(w, r, err)
		return
	}
	writeJSON(w, 200, map[string]any{"preview": preview})
}

func (s *Server) createStudySession(w http.ResponseWriter, r *http.Request) {
	var in createStudySessionInput
	if !decodeJSON(w, r, &in) {
		return
	}
	folderID, ok := studyScope(w, r, in.FolderID)
	if !ok {
		return
	}
	session, fields, err := s.study.Create(r.Context(), currentUserID(r), in.ScopeType, folderID, in.Count)
	if fields != nil {
		writeError(w, r, 422, "validation_failed", "Проверьте параметры", fields)
		return
	}
	if errors.Is(err, study.ErrFolderNotFound) {
		writeError(w, r, 404, "folder_not_found", "Папка не найдена", nil)
		return
	}
	if errors.Is(err, study.ErrEmptyScope) {
		writeError(w, r, 409, "empty_study_scope", "В выбранной области нет карточек", nil)
		return
	}
	if err != nil {
		s.internal(w, r, err)
		return
	}
	writeJSON(w, 201, map[string]any{"session": session})
}

func studySessionID(r *http.Request) (uuid.UUID, bool) {
	id, err := uuid.Parse(chi.URLParam(r, "sessionId"))
	return id, err == nil
}

func studyPosition(r *http.Request) (int, bool) {
	position, err := strconv.Atoi(chi.URLParam(r, "position"))
	return position, err == nil && position > 0
}

func (s *Server) activeStudySession(w http.ResponseWriter, r *http.Request) {
	state, err := s.study.Active(r.Context(), currentUserID(r))
	if errors.Is(err, study.ErrNoActiveSession) {
		w.WriteHeader(204)
		return
	}
	if err != nil {
		s.internal(w, r, err)
		return
	}
	writeJSON(w, 200, map[string]any{"session": state})
}

func (s *Server) getStudySession(w http.ResponseWriter, r *http.Request) {
	id, ok := studySessionID(r)
	if !ok {
		writeError(w, r, 404, "study_session_not_found", "Сессия не найдена", nil)
		return
	}
	state, err := s.study.Get(r.Context(), currentUserID(r), id)
	if s.writeStudyRunError(w, r, err) {
		return
	}
	writeJSON(w, 200, map[string]any{"session": state})
}

func (s *Server) studySessionResult(w http.ResponseWriter, r *http.Request) {
	id, ok := studySessionID(r)
	if !ok {
		writeError(w, r, 404, "study_session_not_found", "Сессия не найдена", nil)
		return
	}
	result, err := s.study.Result(r.Context(), currentUserID(r), id)
	if errors.Is(err, study.ErrResultUnavailable) {
		writeError(w, r, 409, "study_result_unavailable", "Итог доступен после завершения сессии", nil)
		return
	}
	if s.writeStudyRunError(w, r, err) {
		return
	}
	writeJSON(w, 200, map[string]any{"result": result})
}

func (s *Server) revealStudyPosition(w http.ResponseWriter, r *http.Request) {
	id, ok := studySessionID(r)
	position, valid := studyPosition(r)
	if !ok || !valid {
		writeError(w, r, 404, "study_session_not_found", "Сессия или позиция не найдена", nil)
		return
	}
	state, err := s.study.Reveal(r.Context(), currentUserID(r), id, position)
	if s.writeStudyRunError(w, r, err) {
		return
	}
	writeJSON(w, 200, map[string]any{"session": state})
}

type answerStudyPositionInput struct {
	Rating    string `json:"rating"`
	RequestID string `json:"requestId"`
}

func (s *Server) answerStudyPosition(w http.ResponseWriter, r *http.Request) {
	id, ok := studySessionID(r)
	position, valid := studyPosition(r)
	if !ok || !valid {
		writeError(w, r, 404, "study_session_not_found", "Сессия или позиция не найдена", nil)
		return
	}
	var in answerStudyPositionInput
	if !decodeJSON(w, r, &in) {
		return
	}
	var requestFields study.FieldErrors
	if in.RequestID == "" {
		requestFields = study.FieldErrors{"requestId": "Укажите идентификатор запроса"}
	} else if _, parseErr := uuid.Parse(in.RequestID); parseErr != nil {
		requestFields = study.FieldErrors{"requestId": "Некорректный идентификатор запроса"}
	}
	if requestFields != nil {
		writeError(w, r, 422, "validation_failed", "Проверьте параметры", requestFields)
		return
	}
	state, fields, err := s.study.Answer(r.Context(), currentUserID(r), id, position, in.Rating)
	if fields != nil {
		writeError(w, r, 422, "validation_failed", "Проверьте параметры", fields)
		return
	}
	if s.writeStudyRunError(w, r, err) {
		return
	}
	writeJSON(w, 200, map[string]any{"session": state})
}

func (s *Server) abandonStudySession(w http.ResponseWriter, r *http.Request) {
	id, ok := studySessionID(r)
	if !ok {
		writeError(w, r, 404, "study_session_not_found", "Сессия не найдена", nil)
		return
	}
	if s.writeStudyRunError(w, r, s.study.Abandon(r.Context(), currentUserID(r), id)) {
		return
	}
	w.WriteHeader(204)
}

func (s *Server) writeStudyRunError(w http.ResponseWriter, r *http.Request, err error) bool {
	if err == nil {
		return false
	}
	switch {
	case errors.Is(err, study.ErrSessionNotFound):
		writeError(w, r, 404, "study_session_not_found", "Сессия не найдена", nil)
	case errors.Is(err, study.ErrNotRevealed):
		writeError(w, r, 409, "study_position_not_revealed", "Сначала покажите ответ", nil)
	case errors.Is(err, study.ErrAnswerConflict):
		writeError(w, r, 409, "study_answer_conflict", "Для карточки уже сохранён другой ответ", nil)
	case errors.Is(err, study.ErrPositionConflict):
		writeError(w, r, 409, "study_position_conflict", "Позиция больше не является текущей", nil)
	case errors.Is(err, study.ErrSessionInactive):
		writeError(w, r, 409, "study_session_inactive", "Сессия уже завершена", nil)
	default:
		s.internal(w, r, err)
	}
	return true
}

func (s *Server) authenticated(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		cookie, err := r.Cookie(s.cfg.CookieName)
		if err != nil {
			writeError(w, r, 401, "unauthenticated", "Требуется вход", nil)
			return
		}
		user, err := s.auth.CurrentUser(r.Context(), cookie.Value)
		if errors.Is(err, auth.ErrNotFound) {
			s.clearCookie(w)
			writeError(w, r, 401, "unauthenticated", "Требуется вход", nil)
			return
		}
		if err != nil {
			s.internal(w, r, err)
			return
		}
		setUserID(r, user.ID.String())
		next.ServeHTTP(w, r)
	})
}

func currentUserID(r *http.Request) uuid.UUID {
	meta, _ := r.Context().Value(requestIDKey).(*requestMeta)
	if meta == nil {
		return uuid.Nil
	}
	id, _ := uuid.Parse(meta.userID)
	return id
}

type createCardInput struct {
	Front    string  `json:"front"`
	Back     string  `json:"back"`
	FolderID *string `json:"folderId"`
}
type updateCardInput struct {
	Front    *string `json:"front"`
	Back     *string `json:"back"`
	FolderID *string `json:"folderId"`
}

func decodeJSON(w http.ResponseWriter, r *http.Request, out any) bool {
	if mt, _, err := mime.ParseMediaType(r.Header.Get("Content-Type")); err != nil || mt != "application/json" {
		writeError(w, r, 400, "invalid_request", "Ожидается JSON", nil)
		return false
	}
	r.Body = http.MaxBytesReader(w, r.Body, 64<<10)
	d := json.NewDecoder(r.Body)
	d.DisallowUnknownFields()
	if err := d.Decode(out); err != nil {
		writeError(w, r, 400, "invalid_request", "Некорректный JSON", nil)
		return false
	}
	if err := d.Decode(&struct{}{}); err != io.EOF {
		writeError(w, r, 400, "invalid_request", "Некорректный JSON", nil)
		return false
	}
	return true
}

func cardID(r *http.Request) (uuid.UUID, bool) {
	id, err := uuid.Parse(chi.URLParam(r, "cardId"))
	return id, err == nil
}

func (s *Server) listCards(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	var folderID *uuid.UUID
	if q.Has("folderId") {
		parsed, err := uuid.Parse(q.Get("folderId"))
		if err != nil {
			writeError(w, r, 422, "validation_failed", "Проверьте параметры", map[string]string{"folderId": "Некорректная папка"})
			return
		}
		folderID = &parsed
	}
	limit := 50
	if raw := q.Get("limit"); raw != "" {
		n, err := strconv.Atoi(raw)
		if err != nil || n < 1 || n > 100 {
			writeError(w, r, 422, "validation_failed", "Проверьте параметры", map[string]string{"limit": "От 1 до 100"})
			return
		}
		limit = n
	}
	items, next, err := s.cards.List(r.Context(), currentUserID(r), folderID, q.Get("cursor"), limit)
	if errors.Is(err, cards.ErrInvalidCursor) {
		writeError(w, r, 400, "invalid_cursor", "Некорректный курсор", nil)
		return
	}
	if errors.Is(err, cards.ErrFolderNotFound) {
		writeError(w, r, 404, "folder_not_found", "Папка не найдена", nil)
		return
	}
	if err != nil {
		s.internal(w, r, err)
		return
	}
	var nextCursor *string
	if next != "" {
		nextCursor = &next
	}
	writeJSON(w, 200, map[string]any{"items": items, "nextCursor": nextCursor})
}
func (s *Server) createCard(w http.ResponseWriter, r *http.Request) {
	var in createCardInput
	if !decodeJSON(w, r, &in) {
		return
	}
	folderID, ok := optionalUUID(w, r, in.FolderID, "folderId")
	if !ok {
		return
	}
	card, fields, err := s.cards.Create(r.Context(), currentUserID(r), folderID, in.Front, in.Back)
	if fields != nil {
		writeError(w, r, 422, "validation_failed", "Проверьте введённые данные", fields)
		return
	}
	if errors.Is(err, cards.ErrFolderNotFound) {
		writeError(w, r, 404, "folder_not_found", "Папка не найдена", nil)
		return
	}
	if err != nil {
		s.internal(w, r, err)
		return
	}
	writeJSON(w, 201, map[string]any{"card": card})
}
func (s *Server) getCard(w http.ResponseWriter, r *http.Request) {
	id, ok := cardID(r)
	if !ok {
		writeError(w, r, 404, "card_not_found", "Карточка не найдена", nil)
		return
	}
	card, err := s.cards.Get(r.Context(), currentUserID(r), id)
	if errors.Is(err, cards.ErrNotFound) {
		writeError(w, r, 404, "card_not_found", "Карточка не найдена", nil)
		return
	}
	if err != nil {
		s.internal(w, r, err)
		return
	}
	writeJSON(w, 200, map[string]any{"card": card})
}
func (s *Server) updateCard(w http.ResponseWriter, r *http.Request) {
	id, ok := cardID(r)
	if !ok {
		writeError(w, r, 404, "card_not_found", "Карточка не найдена", nil)
		return
	}
	var in updateCardInput
	if !decodeJSON(w, r, &in) {
		return
	}
	folderID, valid := optionalUUID(w, r, in.FolderID, "folderId")
	if !valid {
		return
	}
	card, fields, err := s.cards.Update(r.Context(), currentUserID(r), id, in.Front, in.Back, folderID)
	if fields != nil {
		writeError(w, r, 422, "validation_failed", "Проверьте введённые данные", fields)
		return
	}
	if errors.Is(err, cards.ErrNotFound) {
		writeError(w, r, 404, "card_not_found", "Карточка не найдена", nil)
		return
	}
	if errors.Is(err, cards.ErrFolderNotFound) {
		writeError(w, r, 404, "folder_not_found", "Папка не найдена", nil)
		return
	}
	if err != nil {
		s.internal(w, r, err)
		return
	}
	writeJSON(w, 200, map[string]any{"card": card})
}

func optionalUUID(w http.ResponseWriter, r *http.Request, value *string, field string) (*uuid.UUID, bool) {
	if value == nil {
		return nil, true
	}
	id, err := uuid.Parse(*value)
	if err != nil {
		writeError(w, r, 422, "validation_failed", "Проверьте введённые данные", map[string]string{field: "Некорректный идентификатор"})
		return nil, false
	}
	return &id, true
}

type createFolderInput struct {
	ParentID string `json:"parentId"`
	Name     string `json:"name"`
}

type updateFolderInput struct {
	ParentID *string `json:"parentId"`
	Name     *string `json:"name"`
}

func folderID(r *http.Request) (uuid.UUID, bool) {
	id, err := uuid.Parse(chi.URLParam(r, "folderId"))
	return id, err == nil
}

func (s *Server) folderTree(w http.ResponseWriter, r *http.Request) {
	root, err := s.folders.Tree(r.Context(), currentUserID(r))
	if err != nil {
		s.internal(w, r, err)
		return
	}
	writeJSON(w, 200, map[string]any{"root": root})
}

func (s *Server) createFolder(w http.ResponseWriter, r *http.Request) {
	var in createFolderInput
	if !decodeJSON(w, r, &in) {
		return
	}
	parentID, err := uuid.Parse(in.ParentID)
	if err != nil {
		writeError(w, r, 422, "validation_failed", "Проверьте введённые данные", map[string]string{"parentId": "Некорректный идентификатор"})
		return
	}
	folder, fields, err := s.folders.Create(r.Context(), currentUserID(r), parentID, in.Name)
	if fields != nil {
		writeError(w, r, 422, "validation_failed", "Проверьте введённые данные", fields)
		return
	}
	if s.writeFolderError(w, r, err) {
		return
	}
	writeJSON(w, 201, map[string]any{"folder": folder})
}

func (s *Server) updateFolder(w http.ResponseWriter, r *http.Request) {
	id, ok := folderID(r)
	if !ok {
		writeError(w, r, 404, "folder_not_found", "Папка не найдена", nil)
		return
	}
	var in updateFolderInput
	if !decodeJSON(w, r, &in) {
		return
	}
	parentID, valid := optionalUUID(w, r, in.ParentID, "parentId")
	if !valid {
		return
	}
	folder, fields, err := s.folders.Update(r.Context(), currentUserID(r), id, in.Name, parentID)
	if fields != nil {
		writeError(w, r, 422, "validation_failed", "Проверьте введённые данные", fields)
		return
	}
	if s.writeFolderError(w, r, err) {
		return
	}
	writeJSON(w, 200, map[string]any{"folder": folder})
}

func (s *Server) deleteFolder(w http.ResponseWriter, r *http.Request) {
	id, ok := folderID(r)
	if !ok {
		writeError(w, r, 404, "folder_not_found", "Папка не найдена", nil)
		return
	}
	if s.writeFolderError(w, r, s.folders.Delete(r.Context(), currentUserID(r), id)) {
		return
	}
	w.WriteHeader(204)
}

func (s *Server) writeFolderError(w http.ResponseWriter, r *http.Request, err error) bool {
	if err == nil {
		return false
	}
	switch {
	case errors.Is(err, folders.ErrNotFound):
		writeError(w, r, 404, "folder_not_found", "Папка не найдена", nil)
	case errors.Is(err, folders.ErrInboxImmutable):
		writeError(w, r, 403, "inbox_immutable", "Inbox нельзя изменить", nil)
	case errors.Is(err, folders.ErrNameTaken):
		writeError(w, r, 409, "folder_name_taken", "Название уже используется", map[string]string{"name": "Название уже используется"})
	case errors.Is(err, folders.ErrCycle):
		writeError(w, r, 409, "folder_cycle", "Нельзя переместить папку в её поддерево", nil)
	case errors.Is(err, folders.ErrDepthExceeded):
		writeError(w, r, 422, "folder_depth_exceeded", "Максимальная глубина — 10 уровней", map[string]string{"parentId": "Максимальная глубина — 10 уровней"})
	case errors.Is(err, folders.ErrNotEmpty):
		writeError(w, r, 409, "folder_not_empty", "Сначала перенесите или удалите содержимое папки", nil)
	default:
		s.internal(w, r, err)
	}
	return true
}
func (s *Server) deleteCard(w http.ResponseWriter, r *http.Request) {
	id, ok := cardID(r)
	if !ok {
		writeError(w, r, 404, "card_not_found", "Карточка не найдена", nil)
		return
	}
	err := s.cards.Delete(r.Context(), currentUserID(r), id)
	if errors.Is(err, cards.ErrNotFound) {
		writeError(w, r, 404, "card_not_found", "Карточка не найдена", nil)
		return
	}
	if err != nil {
		s.internal(w, r, err)
		return
	}
	w.WriteHeader(204)
}

func (s *Server) register(w http.ResponseWriter, r *http.Request) {
	in, ok := decodeCredentials(w, r)
	if !ok {
		return
	}
	u, t, fields, e := s.auth.Register(r.Context(), in.Login, in.Password)
	if fields != nil {
		writeError(w, r, 422, "validation_failed", "Проверьте введённые данные", fields)
		return
	}
	if errors.Is(e, auth.ErrLoginTaken) {
		writeError(w, r, 409, "login_taken", "Этот логин уже занят", nil)
		return
	}
	if e != nil {
		s.internal(w, r, e)
		return
	}
	s.setCookie(w, t)
	setUserID(r, u.ID.String())
	writeJSON(w, 201, map[string]any{"user": u})
}
func (s *Server) login(w http.ResponseWriter, r *http.Request) {
	in, ok := decodeCredentials(w, r)
	if !ok {
		return
	}
	if fields := auth.ValidateCredentials(in.Login, in.Password); fields != nil {
		writeError(w, r, 422, "validation_failed", "Проверьте введённые данные", fields)
		return
	}
	key := remoteIP(r) + "\x00" + auth.NormalizeLogin(in.Login)
	if allowed, retry := s.limiter.Allow(key); !allowed {
		w.Header().Set("Retry-After", fmt.Sprint(retry))
		writeError(w, r, 429, "login_rate_limited", "Слишком много попыток. Попробуйте позже", nil)
		return
	}
	u, t, _, e := s.auth.Login(r.Context(), in.Login, in.Password)
	if errors.Is(e, auth.ErrNotFound) {
		s.limiter.Failure(key)
		writeError(w, r, 401, "invalid_credentials", "Неверный логин или пароль", nil)
		return
	}
	if e != nil {
		s.internal(w, r, e)
		return
	}
	s.limiter.Success(key)
	s.setCookie(w, t)
	setUserID(r, u.ID.String())
	writeJSON(w, 200, map[string]any{"user": u})
}
func (s *Server) logout(w http.ResponseWriter, r *http.Request) {
	c, _ := r.Cookie(s.cfg.CookieName)
	if c != nil {
		if e := s.auth.Logout(r.Context(), c.Value); e != nil {
			s.internal(w, r, e)
			return
		}
	}
	s.clearCookie(w)
	w.WriteHeader(204)
}
func (s *Server) me(w http.ResponseWriter, r *http.Request) {
	c, e := r.Cookie(s.cfg.CookieName)
	if e != nil {
		writeError(w, r, 401, "unauthenticated", "Требуется вход", nil)
		return
	}
	u, e := s.auth.CurrentUser(r.Context(), c.Value)
	if errors.Is(e, auth.ErrNotFound) {
		s.clearCookie(w)
		writeError(w, r, 401, "unauthenticated", "Требуется вход", nil)
		return
	}
	if e != nil {
		s.internal(w, r, e)
		return
	}
	setUserID(r, u.ID.String())
	writeJSON(w, 200, map[string]any{"user": u})
}
func (s *Server) live(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, 200, map[string]string{"status": "ok"})
}
func (s *Server) ready(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), time.Second)
	defer cancel()
	if e := s.pool.Ping(ctx); e != nil {
		writeError(w, r, 503, "not_ready", "Сервис не готов", nil)
		return
	}
	writeJSON(w, 200, map[string]string{"status": "ok"})
}

type credentials struct {
	Login    string `json:"login"`
	Password string `json:"password"`
}

func decodeCredentials(w http.ResponseWriter, r *http.Request) (credentials, bool) {
	if mt, _, e := mime.ParseMediaType(r.Header.Get("Content-Type")); e != nil || mt != "application/json" {
		writeError(w, r, 400, "invalid_request", "Ожидается JSON", nil)
		return credentials{}, false
	}
	r.Body = http.MaxBytesReader(w, r.Body, 16<<10)
	d := json.NewDecoder(r.Body)
	d.DisallowUnknownFields()
	var in credentials
	if e := d.Decode(&in); e != nil {
		writeError(w, r, 400, "invalid_request", "Некорректный JSON", nil)
		return in, false
	}
	if e := d.Decode(&struct{}{}); e != io.EOF {
		writeError(w, r, 400, "invalid_request", "Некорректный JSON", nil)
		return in, false
	}
	return in, true
}
func (s *Server) csrf(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		origin := r.Header.Get("Origin")
		site := r.Header.Get("Sec-Fetch-Site")
		if origin == "" && site == "" {
			writeError(w, r, 403, "csrf_rejected", "Запрос отклонён", nil)
			return
		}
		if origin != "" && origin != s.cfg.TrustedOrigin {
			writeError(w, r, 403, "csrf_rejected", "Запрос отклонён", nil)
			return
		}
		if origin != "" && site == "none" {
			writeError(w, r, 403, "csrf_rejected", "Запрос отклонён", nil)
			return
		}
		if site != "" && site != "same-origin" && site != "none" {
			writeError(w, r, 403, "csrf_rejected", "Запрос отклонён", nil)
			return
		}
		next.ServeHTTP(w, r)
	})
}
func (s *Server) setCookie(w http.ResponseWriter, token string) {
	http.SetCookie(w, &http.Cookie{Name: s.cfg.CookieName, Value: token, Path: "/", HttpOnly: true, Secure: s.cfg.Environment == "production", SameSite: http.SameSiteLaxMode, MaxAge: int(s.cfg.SessionTTL.Seconds()), Expires: s.auth.Clock.Now().UTC().Add(s.cfg.SessionTTL)})
}
func (s *Server) clearCookie(w http.ResponseWriter) {
	http.SetCookie(w, &http.Cookie{Name: s.cfg.CookieName, Path: "/", HttpOnly: true, Secure: s.cfg.Environment == "production", SameSite: http.SameSiteLaxMode, MaxAge: -1, Expires: time.Unix(1, 0).UTC()})
}
func (s *Server) internal(w http.ResponseWriter, r *http.Request, e error) {
	s.log.Error("request failed", "requestId", requestID(r), "error", e)
	writeError(w, r, 500, "internal_error", "Внутренняя ошибка", nil)
}

func (s *Server) requestContext(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		id := uuid.NewString()
		w.Header().Set("X-Request-ID", id)
		meta := &requestMeta{id: id}
		next.ServeHTTP(w, r.WithContext(context.WithValue(r.Context(), requestIDKey, meta)))
	})
}
func (s *Server) security(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("Referrer-Policy", "no-referrer")
		w.Header().Set("Permissions-Policy", "camera=(), microphone=(), geolocation=()")
		w.Header().Set("Content-Security-Policy", "default-src 'self'; script-src 'self'; style-src 'self'; img-src 'self'; connect-src 'self'; base-uri 'none'; frame-ancestors 'none'; form-action 'self'")
		if s.cfg.Environment == "production" {
			w.Header().Set("Strict-Transport-Security", "max-age=31536000; includeSubDomains")
		}
		next.ServeHTTP(w, r)
	})
}
func noStore(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "no-store")
		next.ServeHTTP(w, r)
	})
}

type recorder struct {
	http.ResponseWriter
	status int
}

func (r *recorder) WriteHeader(n int) { r.status = n; r.ResponseWriter.WriteHeader(n) }
func (r *recorder) Write(b []byte) (int, error) {
	if r.status == 0 {
		r.WriteHeader(200)
	}
	return r.ResponseWriter.Write(b)
}
func (s *Server) accessLog(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		rw := &recorder{ResponseWriter: w}
		defer func() {
			status := rw.status
			if status == 0 {
				status = 500
			}
			route := chi.RouteContext(r.Context()).RoutePattern()
			attributes := []any{"requestId", requestID(r), "method", r.Method, "route", route, "status", status, "durationMs", time.Since(start).Milliseconds()}
			if meta, _ := r.Context().Value(requestIDKey).(*requestMeta); meta != nil && meta.userID != "" {
				attributes = append(attributes, "userId", meta.userID)
			}
			s.log.Info("http request", attributes...)
		}()
		next.ServeHTTP(rw, r)
	})
}
func (s *Server) recoverer(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if v := recover(); v != nil {
				s.log.Error("panic recovered", "requestId", requestID(r), "panic", fmt.Sprint(v))
				writeError(w, r, 500, "internal_error", "Внутренняя ошибка", nil)
			}
		}()
		next.ServeHTTP(w, r)
	})
}
func (s *Server) frontend() http.Handler {
	root := "web/dist"
	files := http.FileServer(http.Dir(root))
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/app" || strings.HasPrefix(r.URL.Path, "/app/") {
			w.Header().Set("Cache-Control", "no-store")
		}
		if strings.HasPrefix(r.URL.Path, "/api/") || strings.HasPrefix(r.URL.Path, "/health/") {
			http.NotFound(w, r)
			return
		}
		p := filepath.Join(root, filepath.Clean(r.URL.Path))
		if st, e := os.Stat(p); e == nil && !st.IsDir() {
			files.ServeHTTP(w, r)
			return
		}
		if _, e := fs.Stat(os.DirFS(root), "index.html"); e != nil {
			http.Error(w, "frontend is not built", 503)
			return
		}
		http.ServeFile(w, r, filepath.Join(root, "index.html"))
	})
}
func remoteIP(r *http.Request) string {
	h, _, e := net.SplitHostPort(r.RemoteAddr)
	if e == nil {
		return h
	}
	return r.RemoteAddr
}
func requestID(r *http.Request) string {
	meta, _ := r.Context().Value(requestIDKey).(*requestMeta)
	if meta == nil {
		return ""
	}
	return meta.id
}
func setUserID(r *http.Request, userID string) {
	if meta, _ := r.Context().Value(requestIDKey).(*requestMeta); meta != nil {
		meta.userID = userID
	}
}
func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}
func writeError(w http.ResponseWriter, r *http.Request, status int, code, message string, fields any) {
	e := map[string]any{"code": code, "message": message, "requestId": requestID(r)}
	if fields != nil {
		e["fields"] = fields
	}
	writeJSON(w, status, map[string]any{"error": e})
}
