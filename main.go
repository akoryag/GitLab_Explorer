package main

import (
	"encoding/json"
	"html/template"
	"log"
	"net/http"
	"strconv"
	"strings"

	gitlab "gitlab.com/gitlab-org/api/client-go"
)

type ProjectInfo struct {
	Name string
	ID   int
	Refs []string // ветки и теги (теги с префиксом tag:)
}

type GroupInfo struct {
	Name     string
	ID       int
	Path     string
	Projects []ProjectInfo
}

type PageData struct {
	Groups []GroupInfo
	Error  string
	Token  string
}

type PipelineJob struct {
	ID     int    `json:"id"`
	Name   string `json:"name"`
	Status string `json:"status"`
	Stage  string `json:"stage"`
}

type BridgeInfo struct {
	ID             int           `json:"id"`
	Name           string        `json:"name"`
	Status         string        `json:"status"`
	DownstreamJobs []PipelineJob `json:"downstream_jobs"`
}

type PipelineInfo struct {
	ID      int           `json:"id"`
	Ref     string        `json:"ref"`
	Jobs    []PipelineJob `json:"jobs"`
	Bridges []BridgeInfo  `json:"bridges"`
	Error   string        `json:"error"`
}

var (
	pageTmpl     *template.Template
	currentToken string
)

func main() {
	// Загружаем шаблон из файла
	var err error
	pageTmpl, err = template.ParseFiles("templates/page.html")
	if err != nil {
		log.Fatal("Ошибка загрузки шаблона:", err)
	}
	http.HandleFunc("/", handler)
	http.HandleFunc("/pipeline", pipelineHandler)
	http.HandleFunc("/playjob", playJobHandler)
	log.Println("Сервер запущен на http://localhost:8080")
	log.Fatal(http.ListenAndServe(":8080", nil))
}

func handler(w http.ResponseWriter, r *http.Request) {
	data := PageData{}

	if r.Method == http.MethodPost {
		token := strings.TrimSpace(r.FormValue("token"))
		data.Token = token

		if token == "" {
			data.Error = "Введите токен"
			_ = pageTmpl.Execute(w, data)
			return
		}
		currentToken = token

		client, err := gitlab.NewClient(currentToken, gitlab.WithBaseURL("https://gitlab.ru/api/v4"))
		if err != nil {
			data.Error = "Ошибка создания клиента: " + err.Error()
			_ = pageTmpl.Execute(w, data)
			return
		}

		var rootGroupID = 0
		rootGroup, _, err := client.Groups.GetGroup(rootGroupID, nil)
		if err != nil {
			data.Error = "Ошибка получения группы: " + err.Error()
			_ = pageTmpl.Execute(w, data)
			return
		}

		allGroups := []*gitlab.Group{rootGroup}
		subgroups, _, err := client.Groups.ListDescendantGroups(rootGroupID, &gitlab.ListDescendantGroupsOptions{})
		if err == nil {
			allGroups = append(allGroups, subgroups...)
		}

		for _, g := range allGroups {
			grp := GroupInfo{Name: g.Name, ID: g.ID, Path: g.FullPath}

			projects, _, err := client.Groups.ListGroupProjects(g.ID, &gitlab.ListGroupProjectsOptions{})
			if err != nil {
				log.Printf("Ошибка получения проектов группы %s: %v", g.FullPath, err)
				continue
			}

			for _, p := range projects {
				prj := ProjectInfo{Name: p.Name, ID: p.ID}

				branches, _, _ := client.Branches.ListBranches(p.ID, &gitlab.ListBranchesOptions{})
				for _, b := range branches {
					prj.Refs = append(prj.Refs, b.Name)
				}

				tags, _, _ := client.Tags.ListTags(p.ID, &gitlab.ListTagsOptions{})
				for _, t := range tags {
					prj.Refs = append(prj.Refs, "tag:"+t.Name)
				}

				grp.Projects = append(grp.Projects, prj)
			}

			data.Groups = append(data.Groups, grp)
		}
	}

	_ = pageTmpl.Execute(w, data)
}

func pipelineHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	if currentToken == "" {
		json.NewEncoder(w).Encode(PipelineInfo{Error: "Токен не задан"})
		return
	}

	projectID := r.URL.Query().Get("project_id")
	if projectID == "" {
		json.NewEncoder(w).Encode(PipelineInfo{Error: "ID проекта не предоставлен"})
		return
	}

	ref := r.URL.Query().Get("ref")
	if ref == "" {
		json.NewEncoder(w).Encode(PipelineInfo{Error: "Ref не предоставлен"})
		return
	}

	// убираем префикс "tag:"
	if after, ok := strings.CutPrefix(ref, "tag:"); ok {
		ref = after
	}

	client, err := gitlab.NewClient(currentToken, gitlab.WithBaseURL("https://gitlab.ru/api/v4"))
	if err != nil {
		json.NewEncoder(w).Encode(PipelineInfo{Error: "Ошибка создания клиента: " + err.Error()})
		return
	}

	// берем последний пайплайн для рефа
	opts := &gitlab.ListProjectPipelinesOptions{
		Ref:         gitlab.Ptr(ref),
		Sort:        gitlab.Ptr("desc"),
		OrderBy:     gitlab.Ptr("id"),
		ListOptions: gitlab.ListOptions{PerPage: 1, Page: 1},
	}
	pipelines, _, err := client.Pipelines.ListProjectPipelines(projectID, opts)
	if err != nil {
		json.NewEncoder(w).Encode(PipelineInfo{Error: "Ошибка получения пайплайнов: " + err.Error()})
		return
	}
	if len(pipelines) == 0 {
		json.NewEncoder(w).Encode(PipelineInfo{Error: "Нет пайплайнов для этого рефа"})
		return
	}
	pipeline := pipelines[0]

	// список джобов
	jobOpts := &gitlab.ListJobsOptions{}
	jobs, _, err := client.Jobs.ListPipelineJobs(projectID, pipeline.ID, jobOpts)
	if err != nil {
		json.NewEncoder(w).Encode(PipelineInfo{Error: "Ошибка получения джобов: " + err.Error()})
		return
	}
	var pipelineJobs []PipelineJob
	for _, j := range jobs {
		pipelineJobs = append(pipelineJobs, PipelineJob{
			ID:     j.ID,
			Name:   j.Name,
			Status: j.Status,
			Stage:  j.Stage,
		})
	}

	// список bridges
	bridges, _, err := client.Jobs.ListPipelineBridges(projectID, pipeline.ID, &gitlab.ListJobsOptions{})
	if err != nil {
		json.NewEncoder(w).Encode(PipelineInfo{Error: "Ошибка получения bridges: " + err.Error()})
		return
	}
	var pipelineBridges []BridgeInfo
	for _, bridge := range bridges {
		b := BridgeInfo{
			ID:     bridge.ID,
			Name:   bridge.Name,
			Status: bridge.Status,
		}

		// загружаем downstream jobs если есть
		if bridge.DownstreamPipeline != nil {
			downstreamJobs, _, err := client.Jobs.ListPipelineJobs(
				bridge.DownstreamPipeline.ProjectID,
				bridge.DownstreamPipeline.ID,
				&gitlab.ListJobsOptions{},
			)
			if err == nil {
				for _, job := range downstreamJobs {
					b.DownstreamJobs = append(b.DownstreamJobs, PipelineJob{
						ID:     job.ID,
						Name:   job.Name,
						Status: job.Status,
						Stage:  job.Stage,
					})
				}
			}
		}

		pipelineBridges = append(pipelineBridges, b)
	}

	// возвращаем данные
	json.NewEncoder(w).Encode(PipelineInfo{
		ID:      pipeline.ID,
		Ref:     ref,
		Jobs:    pipelineJobs,
		Bridges: pipelineBridges,
	})
}

func playJobHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	if currentToken == "" {
		json.NewEncoder(w).Encode(PipelineInfo{Error: "Токен не задан"})
		return
	}

	projectID := r.URL.Query().Get("project_id")
	if projectID == "" {
		json.NewEncoder(w).Encode(map[string]string{"error": "ID проекта не предоставлен"})
		return
	}

	jobID := r.URL.Query().Get("job_id")
	if jobID == "" {
		json.NewEncoder(w).Encode(map[string]string{"error": "ID джобы не предоставлен"})
		return
	}

	client, err := gitlab.NewClient(currentToken, gitlab.WithBaseURL("https://gitlab.ru/api/v4"))
	if err != nil {
		json.NewEncoder(w).Encode(map[string]string{"error": "Ошибка создания клиента: " + err.Error()})
		return
	}

	// Запускаем джобу
	jobIDInt, _ := strconv.Atoi(jobID)
	projectIDInt, _ := strconv.Atoi(projectID)

	_, _, err = client.Jobs.PlayJob(projectIDInt, jobIDInt, &gitlab.PlayJobOptions{})
	if err != nil {
		json.NewEncoder(w).Encode(map[string]string{"error": "Ошибка запуска джобы: " + err.Error()})
		return
	}

	json.NewEncoder(w).Encode(map[string]string{"status": "success"})
}
