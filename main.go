package main

import (
	"bufio"
	"fmt"
	"log"
	"os"
	"strings"

	gitlab "gitlab.com/gitlab-org/api/client-go"
)

func main() {
	reader := bufio.NewReader(os.Stdin)

	fmt.Printf("Gitlab OAuth token: ")
	token, _ := reader.ReadString('\n')
	token = strings.TrimSpace(token)

	// fmt.Print("ID группы: ")
	// groupID, _ := reader.ReadString('\n')
	// groupID = strings.TrimSpace(groupID)
	var groupID = 0

	client, err := gitlab.NewClient(token, gitlab.WithBaseURL("https://gitlab.ru/api/v4"))
	if err != nil {
		fmt.Printf("Ошибка создания клиента: %v\n", err)
		return
	}

	rootGroup, _, err := client.Groups.GetGroup(groupID, nil)
	if err != nil {
		fmt.Printf("Ошибка получения группы: %v\n", err)
		return
	}

	GetProjectsAndBranches(client, rootGroup)

	subgroups, _, err := client.Groups.ListDescendantGroups(groupID, &gitlab.ListDescendantGroupsOptions{})
	if err != nil {
		log.Fatalf("Ошибка получения дочерних групп: %v\n", err)
	}

	for _, g := range subgroups {
		GetProjectsAndBranches(client, g)
	}
}

func GetProjectsAndBranches(client *gitlab.Client, group *gitlab.Group) {
	fmt.Printf("\nГруппа: %s (ID: %d, Path: %s)\n", group.Name, group.ID, group.FullPath)

	projects, _, err := client.Groups.ListGroupProjects(group.ID, &gitlab.ListGroupProjectsOptions{})
	if err != nil {
		fmt.Printf("Ошибка получения проектов группы %s: %v", group.ID, err)
		return
	}

	for _, p := range projects {
		fmt.Printf("	Проект: %s (ID: %d)\n", p.Name, p.ID)

		branches, _, err := client.Branches.ListBranches(p.ID, &gitlab.ListBranchesOptions{})
		if err != nil {
			fmt.Printf("Ошибка получения веток проекта %s: %v", p.PathWithNamespace, err)
			continue
		} else {
			for _, b := range branches {
				fmt.Printf("		Ветка: %s\n", b.Name)
			}
		}

		tags, _, err := client.Tags.ListTags(p.ID, &gitlab.ListTagsOptions{})
		if err != nil {
			fmt.Printf("Ошибка получения тегов проекта %s (ID: %d)\n", p.PathWithNamespace, err)
		} else {
			for _, t := range tags {
				fmt.Printf("			tag: %s\n", t.Name)
			}
		}
	}
}
