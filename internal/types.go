package app

type ProjectInfo struct {
	Name string
	ID   int
	Refs []string
}

type GroupInfo struct {
	Name     string
	ID       int
	Path     string
	Projects []ProjectInfo
	AllRefs  []string
}

type PageData struct {
	Groups []GroupInfo
	Error  string
	Token  string
	Loaded bool
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
