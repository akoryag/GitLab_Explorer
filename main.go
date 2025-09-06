package main

import (
	"bufio"
	"fmt"
	"log"
	"os"
	"sort"
	"strings"

	gitlab "gitlab.com/gitlab-org/api/client-go"
)

type ProjectInfo struct {
	Name    string
	ID      int
	Refs    []string // ветки и теги (теги с префиксом tag:)
	LastTag string   // последний тег
	Pipe    []PipelineInfo
}

type PipelineInfo struct {
	ID      int
	Status  string
	Ref     string
	Bridges []BridgeInfo
}

type BridgeInfo struct {
	ID     int
	Name   string
	Status string
	Jobs   []JobInfo
}

type JobInfo struct {
	ID     int
	Name   string
	Status string
	Stage  string
}

type GroupInfo struct {
	Name     string
	ID       int
	Path     string
	Projects []ProjectInfo
}

func main() {
	reader := bufio.NewReader(os.Stdin)

	fmt.Printf("Gitlab OAuth token: ")
	token, _ := reader.ReadString('\n')
	token = strings.TrimSpace(token)

	// fmt.Print("ID группы: ")
	// groupID, _ := reader.ReadString('\n')
	// groupID = strings.TrimSpace(groupID)

	client, err := gitlab.NewClient(token, gitlab.WithBaseURL("https://gitlab.ru/api/v4"))
	if err != nil {
		log.Fatalf("Ошибка создания клиента: %v", err)
	}

	// корневая группа
	rootGroupID := 0
	rootGroup, _, err := client.Groups.GetGroup(rootGroupID, nil)
	if err != nil {
		log.Fatalf("Ошибка получения группы: %v", err)
	}

	allGroups := []*gitlab.Group{rootGroup}
	subgroups, _, err := client.Groups.ListDescendantGroups(rootGroupID, &gitlab.ListDescendantGroupsOptions{})
	if err == nil {
		allGroups = append(allGroups, subgroups...)
	}

	var groupsInfo []GroupInfo
	for _, g := range allGroups {
		grp := GroupInfo{Name: g.Name, ID: g.ID, Path: g.FullPath}

		projects, _, err := client.Groups.ListGroupProjects(g.ID, &gitlab.ListGroupProjectsOptions{})
		if err != nil {
			log.Printf("Ошибка получения проектов группы %s: %v", g.FullPath, err)
			continue
		}

		for _, p := range projects {
			prj := ProjectInfo{Name: p.Name, ID: p.ID}

			branches, _, _ := client.Branches.ListBranches(p.ID, &gitlab.ListBranchesOptions{
				ListOptions: gitlab.ListOptions{
					PerPage: 10,
					Page:    1,
				},
			})
			for _, b := range branches {
				prj.Refs = append(prj.Refs, b.Name)
			}

			// Получаем все теги и выбираем последний
			tags, _, _ := client.Tags.ListTags(p.ID, &gitlab.ListTagsOptions{
				OrderBy: gitlab.Ptr("updated"),
				Sort:    gitlab.Ptr("desc"),
			})

			if len(tags) > 0 {
				// Сортируем теги по дате создания, чтобы выбрать самый свежий
				sort.Slice(tags, func(i, j int) bool {
					return tags[i].Commit.CreatedAt.After(*tags[j].Commit.CreatedAt)
				})
				prj.LastTag = tags[0].Name
				prj.Refs = append(prj.Refs, "tag:"+tags[0].Name)
			}

			// Добавляем остальные теги (кроме последнего) в общий список
			for i := 1; i < len(tags); i++ {
				prj.Refs = append(prj.Refs, "tag:"+tags[i].Name)
			}

			// Загружаем пайплайны по последнему тегу вместо ветки master
			var refToSearch string
			if prj.LastTag != "" {
				refToSearch = prj.LastTag
			} else {
				refToSearch = "master"
			}

			pipelines, _, _ := client.Pipelines.ListProjectPipelines(p.ID, &gitlab.ListProjectPipelinesOptions{
				Ref: gitlab.Ptr(refToSearch),
				ListOptions: gitlab.ListOptions{
					PerPage: 1,
					Page:    1,
				},
			})

			for _, pipe := range pipelines {
				pipelineInfo := PipelineInfo{
					ID:     pipe.ID,
					Status: pipe.Status,
					Ref:    pipe.Ref,
				}

				// Получаем bridges для пайплайна
				bridges, _, err := client.Jobs.ListPipelineBridges(p.ID, pipe.ID, &gitlab.ListJobsOptions{})
				if err == nil {
					for _, bridge := range bridges {
						bridgeInfo := BridgeInfo{
							ID:     bridge.ID,
							Name:   bridge.Name,
							Status: bridge.Status,
						}

						// Получаем джобы для bridge
						if bridge.DownstreamPipeline != nil {
							jobs, _, err := client.Jobs.ListPipelineJobs(bridge.DownstreamPipeline.ProjectID, bridge.DownstreamPipeline.ID, &gitlab.ListJobsOptions{})
							if err == nil {
								for _, job := range jobs {
									bridgeInfo.Jobs = append(bridgeInfo.Jobs, JobInfo{
										ID:     job.ID,
										Name:   job.Name,
										Status: job.Status,
										Stage:  job.Stage,
									})
								}
							}
						}

						pipelineInfo.Bridges = append(pipelineInfo.Bridges, bridgeInfo)
					}
				}

				prj.Pipe = append(prj.Pipe, pipelineInfo)
			}

			grp.Projects = append(grp.Projects, prj)
		}

		groupsInfo = append(groupsInfo, grp)
	}

	// выводим результаты
	for _, g := range groupsInfo {
		fmt.Printf("Группа: %s (%s)\n", g.Name, g.Path)
		if len(g.Projects) == 0 {
			fmt.Println("  Нет проектов")
		} else {
			for _, p := range g.Projects {
				fmt.Printf("  Проект: %s\n", p.Name)

				// Выводим последний тег отдельно
				if p.LastTag != "" {
					fmt.Printf("    Последний тег: %s\n", p.LastTag)
				} else {
					fmt.Println("    Последний тег: нет")
				}

				if len(p.Refs) == 0 {
					fmt.Println("    Рефов нет")
				} else {
					fmt.Println("    Рефы:")
					for _, r := range p.Refs {
						fmt.Printf("      %s\n", r)
					}
				}

				if len(p.Pipe) == 0 {
					fmt.Println("    Пайплайнов нет")
				} else {
					fmt.Println("    Пайплайны:")
					for _, pipe := range p.Pipe {
						fmt.Printf("      Пайплайн ID: %d, Status: %s, Ref: %s\n",
							pipe.ID, pipe.Status, pipe.Ref)

						if len(pipe.Bridges) > 0 {
							fmt.Println("        Bridges:")
							for _, bridge := range pipe.Bridges {
								fmt.Printf("          Bridge: %s (ID: %d, Status: %s)\n",
									bridge.Name, bridge.ID, bridge.Status)

								if len(bridge.Jobs) > 0 {
									fmt.Println("            Jobs:")
									for _, job := range bridge.Jobs {
										fmt.Printf("              %s (ID: %d, Status: %s, Stage: %s)\n",
											job.Name, job.ID, job.Status, job.Stage)
									}
								} else {
									fmt.Println("            Jobs: нет")
								}
							}
						} else {
							fmt.Println("        Bridges: нет")
						}
					}
				}
				fmt.Println()
			}
		}
		fmt.Println(strings.Repeat("-", 40))
	}
}
