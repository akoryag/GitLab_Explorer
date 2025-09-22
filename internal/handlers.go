package app

import (
	"encoding/json"
	"html/template"
	"net/http"
	"strconv"
	"strings"

	gitlab "gitlab.com/gitlab-org/api/client-go"
)

type AppContext struct {
	Tmpl *template.Template
}

// ---------- Утилиты ----------

func writeJSON(w http.ResponseWriter, status int, payload interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(payload)
}

// ---------- Middleware ----------

// withGitlabClient достаёт client из cookie
func (app *AppContext) withGitlabClient(next func(http.ResponseWriter, *http.Request, *gitlab.Client)) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		tokenCookie, err1 := r.Cookie("gitlab_token")
		urlCookie, err2 := r.Cookie("gitlab_url")

		if err1 != nil || err2 != nil || tokenCookie.Value == "" || urlCookie.Value == "" {
			writeJSON(w, 401, map[string]string{"error": "Не авторизован"})
			return
		}

		client, err := newGitlabClient(tokenCookie.Value, urlCookie.Value)
		if err != nil {
			writeJSON(w, 500, map[string]string{"error": "Ошибка создания клиента"})
			return
		}

		next(w, r, client)
	}
}

// ---------- Handlers ----------

func (app *AppContext) Handler(w http.ResponseWriter, r *http.Request) {
	data := PageData{}

	if r.Method == http.MethodPost {
		token := strings.TrimSpace(r.FormValue("token"))
		gitlabURL := strings.TrimSpace(r.FormValue("gitlabURL"))
		rootGroupIDStr := strings.TrimSpace(r.FormValue("rootGroupID"))

		// сохраняем в cookie
		http.SetCookie(w, &http.Cookie{Name: "gitlab_token", Value: token, Path: "/", HttpOnly: true})
		http.SetCookie(w, &http.Cookie{Name: "gitlab_url", Value: gitlabURL, Path: "/", HttpOnly: true})

		rootGroupID, err := strconv.Atoi(rootGroupIDStr)
		if err != nil {
			data.Error = "Ошибка поиска группы: " + err.Error()
			_ = app.Tmpl.Execute(w, data)
			return
		}

		client, err := newGitlabClient(token, gitlabURL)
		if err != nil {
			data.Error = "Ошибка создания клиента: " + err.Error()
			_ = app.Tmpl.Execute(w, data)
			return
		}

		groups, err := loadGroups(client, rootGroupID)
		if err != nil {
			data.Error = "Ошибка получения групп: " + err.Error()
		} else {
			data.Groups = groups
			data.Token = token
			data.Loaded = true
		}
	}

	_ = app.Tmpl.Execute(w, data)
}

func (app *AppContext) LogoutHandler(w http.ResponseWriter, r *http.Request) {
	// очищаем cookie
	http.SetCookie(w, &http.Cookie{Name: "gitlab_token", Value: "", Path: "/", MaxAge: -1})
	http.SetCookie(w, &http.Cookie{Name: "gitlab_url", Value: "", Path: "/", MaxAge: -1})

	data := PageData{}
	_ = app.Tmpl.Execute(w, data)
}

func (app *AppContext) PipelineHandler() http.HandlerFunc {
	return app.withGitlabClient(func(w http.ResponseWriter, r *http.Request, client *gitlab.Client) {
		projectID := r.URL.Query().Get("project_id")
		ref := r.URL.Query().Get("ref")

		if projectID == "" || ref == "" {
			writeJSON(w, 400, map[string]string{"error": "Не хватает параметров"})
			return
		}

		if after, ok := strings.CutPrefix(ref, "tag:"); ok {
			ref = after
		}

		info, err := loadPipelineInfo(client, projectID, ref)
		if err != nil {
			writeJSON(w, 500, map[string]string{"error": err.Error()})
			return
		}

		writeJSON(w, 200, info)
	})
}

func (app *AppContext) HandleJobAction() http.HandlerFunc {
	return app.withGitlabClient(func(w http.ResponseWriter, r *http.Request, client *gitlab.Client) {
		projectIDStr := r.URL.Query().Get("project_id")
		jobIDStr := r.URL.Query().Get("job_id")
		action := r.URL.Query().Get("action")

		if projectIDStr == "" || jobIDStr == "" || action == "" {
			writeJSON(w, 400, map[string]string{"error": "Не хватает параметров"})
			return
		}

		projectID, _ := strconv.Atoi(projectIDStr)
		jobID, _ := strconv.Atoi(jobIDStr)

		err := executeJobAction(client, projectID, jobID, action)
		if err != nil {
			writeJSON(w, 500, map[string]string{"error": err.Error()})
			return
		}

		writeJSON(w, 200, map[string]string{"status": "success"})
	})
}
