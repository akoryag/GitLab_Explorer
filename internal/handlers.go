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
		rootGroupIDsStr := strings.TrimSpace(r.FormValue("rootGroupID"))

		// сохраняем в cookie
		http.SetCookie(w, &http.Cookie{Name: "gitlab_token", Value: token, Path: "/", HttpOnly: true})
		http.SetCookie(w, &http.Cookie{Name: "gitlab_url", Value: gitlabURL, Path: "/", HttpOnly: true})

		// Разделяем ID по запятой и преобразуем в числа
		groupIDStrs := strings.Split(rootGroupIDsStr, ",")
		var rootGroupIDs []int
		for _, idStr := range groupIDStrs {
			idStr = strings.TrimSpace(idStr)
			if idStr == "" {
				continue
			}
			id, err := strconv.Atoi(idStr)
			if err != nil {
				data.Error = "Ошибка парсинга ID группы: " + err.Error()
				_ = app.Tmpl.Execute(w, data)
				return
			}
			rootGroupIDs = append(rootGroupIDs, id)
		}

		if len(rootGroupIDs) == 0 {
			data.Error = "Не указано ни одного ID группы"
			_ = app.Tmpl.Execute(w, data)
			return
		}

		client, err := newGitlabClient(token, gitlabURL)
		if err != nil {
			data.Error = "Ошибка создания клиента: " + err.Error()
			_ = app.Tmpl.Execute(w, data)
			return
		}

		// Загружаем группы для всех указанных ID
		var allGroups []GroupInfo
		for _, rootGroupID := range rootGroupIDs {
			groups, err := loadGroups(client, rootGroupID)
			if err != nil {
				data.Error = "Ошибка получения групп для ID " + strconv.Itoa(rootGroupID) + ": " + err.Error()
				_ = app.Tmpl.Execute(w, data)
				return
			}
			allGroups = append(allGroups, groups...)
		}

		data.Groups = allGroups
		data.Token = token
		data.Loaded = true
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

func (app *AppContext) PipelineURLHandler() http.HandlerFunc {
	return app.withGitlabClient(func(w http.ResponseWriter, r *http.Request, client *gitlab.Client) {
		projectIDStr := r.URL.Query().Get("project_id")
		ref := r.URL.Query().Get("ref")
		if projectIDStr == "" || ref == "" {
			writeJSON(w, 400, map[string]string{"error": "Не хватает параметров"})
			return

		}
		if after, ok := strings.CutPrefix(ref, "tag:"); ok {
			ref = after
		}

		info, err := loadPipelineInfo(client, projectIDStr, ref)
		if err != nil {
			writeJSON(w, 500, map[string]string{"error": err.Error()})
			return
		}

		project, _, err := client.Projects.GetProject(projectIDStr, nil)
		if err != nil {
			writeJSON(w, 500, map[string]string{"error": "Не удалось получить проект"})
			return
		}

		pipelineURL := strings.TrimSuffix(client.BaseURL().String(), "api/v4/") + project.PathWithNamespace + "/-/pipelines/" + strconv.Itoa(info.ID)
		writeJSON(w, 200, map[string]string{"url": pipelineURL})
	})
}

func (app *AppContext) TagsHandler() http.HandlerFunc {
	return app.withGitlabClient(func(w http.ResponseWriter, r *http.Request, client *gitlab.Client) {
		projectID := r.URL.Query().Get("project_id")
		ref := r.URL.Query().Get("ref")

		if projectID == "" || ref == "" {
			writeJSON(w, 400, map[string]string{"error": "Не хватает параметров"})
			return
		}

		// Получаем теги проекта
		tags, _, err := client.Tags.ListTags(projectID, &gitlab.ListTagsOptions{})
		if err != nil {
			writeJSON(w, 500, map[string]string{"error": "Ошибка получения тегов: " + err.Error()})
			return
		}

		var tagInfos []map[string]string
		for _, tag := range tags {
			tagInfos = append(tagInfos, map[string]string{
				"name":   tag.Name,
				"commit": tag.Commit.ID,
			})
		}

		writeJSON(w, 200, map[string]interface{}{"tags": tagInfos})
	})
}

// Удаление тега
func (app *AppContext) DeleteTagHandler() http.HandlerFunc {
	return app.withGitlabClient(func(w http.ResponseWriter, r *http.Request, client *gitlab.Client) {
		projectID := r.URL.Query().Get("project_id")
		tagName := r.URL.Query().Get("tag_name")

		if projectID == "" || tagName == "" {
			writeJSON(w, 400, map[string]string{"error": "Не хватает параметров"})
			return
		}

		_, err := client.Tags.DeleteTag(projectID, tagName)
		if err != nil {
			writeJSON(w, 500, map[string]string{"error": "Ошибка удаления тега: " + err.Error()})
			return
		}

		writeJSON(w, 200, map[string]string{"status": "success"})
	})
}

func (app *AppContext) CreateTagHandler() http.HandlerFunc {
	return app.withGitlabClient(func(w http.ResponseWriter, r *http.Request, client *gitlab.Client) {
		if r.Method != http.MethodPost {
			writeJSON(w, 405, map[string]string{"error": "Метод не поддерживается"})
			return
		}

		var request struct {
			ProjectID string `json:"project_id"`
			TagName   string `json:"tag_name"`
			Ref       string `json:"ref"`
		}

		if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
			writeJSON(w, 400, map[string]string{"error": "Ошибка парсинга JSON: " + err.Error()})
			return
		}

		if request.ProjectID == "" || request.TagName == "" || request.Ref == "" {
			writeJSON(w, 400, map[string]string{"error": "Не хватает параметров"})
			return
		}

		// Создаем тег
		opts := &gitlab.CreateTagOptions{
			TagName: gitlab.Ptr(request.TagName),
			Ref:     gitlab.Ptr(request.Ref),
		}

		tag, _, err := client.Tags.CreateTag(request.ProjectID, opts)
		if err != nil {
			writeJSON(w, 500, map[string]string{"error": "Ошибка создания тега: " + err.Error()})
			return
		}

		writeJSON(w, 200, map[string]interface{}{
			"status": "success",
			"tag": map[string]string{
				"name":   tag.Name,
				"commit": tag.Commit.ID,
			},
		})
	})
}
