package app

import (
	gitlab "gitlab.com/gitlab-org/api/client-go"
)

func newGitlabClient(token, baseURL string) (*gitlab.Client, error) {
	return gitlab.NewClient(token, gitlab.WithBaseURL(baseURL+"/api/v4"))
}

// -------- Groups & Projects --------

func loadGroups(client *gitlab.Client, rootGroupID int) ([]GroupInfo, error) {
	rootGroup, _, err := client.Groups.GetGroup(rootGroupID, nil)
	if err != nil {
		return nil, err
	}

	allGroups := []*gitlab.Group{rootGroup}
	subgroups, _, err := client.Groups.ListDescendantGroups(rootGroupID, &gitlab.ListDescendantGroupsOptions{})
	if err == nil {
		allGroups = append(allGroups, subgroups...)
	}

	var result []GroupInfo
	for _, g := range allGroups {
		grp := GroupInfo{Name: g.Name, ID: g.ID, Path: g.FullPath}
		allRefs := make(map[string]struct{})
		allBranches := make(map[string]struct{})

		projects, _, err := client.Groups.ListGroupProjects(g.ID, &gitlab.ListGroupProjectsOptions{})
		if err != nil {
			continue
		}

		for _, p := range projects {
			prj := ProjectInfo{Name: p.Name, ID: p.ID}

			// Загружаем ветки
			branches, _, _ := client.Branches.ListBranches(p.ID, &gitlab.ListBranchesOptions{})
			for _, b := range branches {
				prj.Refs = append(prj.Refs, b.Name)
				prj.Branches = append(prj.Branches, b.Name)
				allRefs[b.Name] = struct{}{}
				allBranches[b.Name] = struct{}{}
			}

			// Загружаем теги
			tags, _, _ := client.Tags.ListTags(p.ID, &gitlab.ListTagsOptions{})
			for _, t := range tags {
				tagName := "tag:" + t.Name
				prj.Refs = append(prj.Refs, tagName)
				allRefs[tagName] = struct{}{}
			}

			grp.Projects = append(grp.Projects, prj)
		}

		for ref := range allRefs {
			grp.AllRefs = append(grp.AllRefs, ref)
		}
		for branch := range allBranches {
			grp.AllBranches = append(grp.AllBranches, branch)
		}

		result = append(result, grp)
	}

	return result, nil
}

// -------- Pipelines --------

func loadPipelineInfo(client *gitlab.Client, projectID, ref string) (*PipelineInfo, error) {
	// получаем последний пайплайн
	opts := &gitlab.ListProjectPipelinesOptions{
		Ref:         gitlab.Ptr(ref),
		Sort:        gitlab.Ptr("desc"),
		OrderBy:     gitlab.Ptr("id"),
		ListOptions: gitlab.ListOptions{PerPage: 1, Page: 1},
	}
	pipelines, _, err := client.Pipelines.ListProjectPipelines(projectID, opts)
	if err != nil {
		return nil, err
	}
	if len(pipelines) == 0 {
		return &PipelineInfo{Error: "Нет пайплайнов для этого рефа"}, nil
	}
	pipeline := pipelines[0]

	// джобы
	jobs, _, err := client.Jobs.ListPipelineJobs(projectID, pipeline.ID, &gitlab.ListJobsOptions{})
	if err != nil {
		return nil, err
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

	// bridges
	bridges, _, err := client.Jobs.ListPipelineBridges(projectID, pipeline.ID, &gitlab.ListJobsOptions{})
	if err != nil {
		return nil, err
	}
	var pipelineBridges []BridgeInfo
	for _, bridge := range bridges {
		b := BridgeInfo{
			ID:     bridge.ID,
			Name:   bridge.Name,
			Status: bridge.Status,
		}

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

	return &PipelineInfo{
		ID:      pipeline.ID,
		Ref:     ref,
		Jobs:    pipelineJobs,
		Bridges: pipelineBridges,
	}, nil
}

// -------- Jobs --------

func executeJobAction(client *gitlab.Client, projectID, jobID int, action string) error {
	switch action {
	case "play":
		_, _, err := client.Jobs.PlayJob(projectID, jobID, &gitlab.PlayJobOptions{})
		return err
	case "retry":
		_, _, err := client.Jobs.RetryJob(projectID, jobID)
		return err
	case "cancel":
		_, _, err := client.Jobs.CancelJob(projectID, jobID)
		return err
	default:
		return ErrUnknownAction
	}
}

var ErrUnknownAction = &ActionError{"неизвестное действие"}

type ActionError struct {
	Msg string
}

func (e *ActionError) Error() string { return e.Msg }
