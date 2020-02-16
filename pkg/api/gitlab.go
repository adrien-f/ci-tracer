package api

import (
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/gorilla/mux"
	"github.com/opentracing/opentracing-go"
	log "github.com/sirupsen/logrus"
)

type GitlabInstance struct {
	name string
}

type GitlabAPI struct {
	*API
	instances map[string]*GitlabInstance
}

type pipelineHook struct {
	ObjectKind       string `json:"object_kind"`
	ObjectAttributes struct {
		ID         int      `json:"id"`
		Ref        string   `json:"ref"`
		Tag        bool     `json:"tag"`
		Sha        string   `json:"sha"`
		BeforeSha  string   `json:"before_sha"`
		Source     string   `json:"source"`
		Status     string   `json:"status"`
		Stages     []string `json:"stages"`
		CreatedAt  string   `json:"created_at"`
		FinishedAt string   `json:"finished_at"`
		Duration   int      `json:"duration"`
		Variables  []struct {
			Key   string `json:"key"`
			Value string `json:"value"`
		} `json:"variables"`
	} `json:"object_attributes"`
	MergeRequest struct {
		ID              int    `json:"id"`
		Iid             int    `json:"iid"`
		Title           string `json:"title"`
		SourceBranch    string `json:"source_branch"`
		SourceProjectID int    `json:"source_project_id"`
		TargetBranch    string `json:"target_branch"`
		TargetProjectID int    `json:"target_project_id"`
		State           string `json:"state"`
		MergeStatus     string `json:"merge_status"`
		URL             string `json:"url"`
	} `json:"merge_request"`
	User struct {
		Name      string `json:"name"`
		Username  string `json:"username"`
		AvatarURL string `json:"avatar_url"`
		Email     string `json:"email"`
	} `json:"user"`
	Project struct {
		ID                int         `json:"id"`
		Name              string      `json:"name"`
		Description       string      `json:"description"`
		WebURL            string      `json:"web_url"`
		AvatarURL         interface{} `json:"avatar_url"`
		GitSSHURL         string      `json:"git_ssh_url"`
		GitHTTPURL        string      `json:"git_http_url"`
		Namespace         string      `json:"namespace"`
		VisibilityLevel   int         `json:"visibility_level"`
		PathWithNamespace string      `json:"path_with_namespace"`
		DefaultBranch     string      `json:"default_branch"`
	} `json:"project"`
	Commit struct {
		ID        string    `json:"id"`
		Message   string    `json:"message"`
		Timestamp time.Time `json:"timestamp"`
		URL       string    `json:"url"`
		Author    struct {
			Name  string `json:"name"`
			Email string `json:"email"`
		} `json:"author"`
	} `json:"commit"`
	Builds []pipelineBuild `json:"builds"`
}

type pipelineBuild struct {
	ID           int         `json:"id"`
	Stage        string      `json:"stage"`
	Name         string      `json:"name"`
	Status       string      `json:"status"`
	CreatedAt    string      `json:"created_at"`
	StartedAt    interface{} `json:"started_at"`
	FinishedAt   interface{} `json:"finished_at"`
	When         string      `json:"when"`
	Manual       bool        `json:"manual"`
	AllowFailure bool        `json:"allow_failure"`
	User         struct {
		Name      string `json:"name"`
		Username  string `json:"username"`
		AvatarURL string `json:"avatar_url"`
	} `json:"user"`
	Runner        runner `json:"runner"`
	ArtifactsFile struct {
		Filename interface{} `json:"filename"`
		Size     interface{} `json:"size"`
	} `json:"artifacts_file"`
}

type runner struct {
	ID          int    `json:"id"`
	Description string `json:"description"`
	Active      bool   `json:"active"`
	IsShared    bool   `json:"is_shared"`
}

const GitlabTimeFormat = "2006-01-02 15:04:05 MST"

func NewGitlabAPI(logger *log.Logger, tracer opentracing.Tracer) *GitlabAPI {
	return &GitlabAPI{
		&API{
			logger: logger.WithField("ingester", "gitlab"),
			tracer: tracer,
		},
		map[string]*GitlabInstance{},
	}
}

func (g *GitlabAPI) Register(r *mux.Router) {
	s := r.PathPrefix("/gitlab").Subrouter()
	s.HandleFunc("/{instance}", g.Ingest)
}

func (g *GitlabAPI) Ingest(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	logger := g.logger.WithField("instance", vars["instance"])
	if _, ok := g.instances[vars["instance"]]; !ok {
		w.WriteHeader(http.StatusNotFound)
		return
	}
	eventType := r.Header.Get("X-Gitlab-Event")
	var err error
	switch eventType {
	case "Pipeline Hook":
		err = g.IngestPipelineHook(r, g.instances[vars["instance"]], logger)
		break
	default:
		w.WriteHeader(http.StatusBadRequest)
		return
	}
	if err != nil {
		logger.WithField("error", err).Error("could not process hook")
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusOK)
}

func (g *GitlabAPI) IngestPipelineHook(r *http.Request, instance *GitlabInstance, logger *log.Entry) error {
	var hook pipelineHook
	err := json.NewDecoder(r.Body).Decode(&hook)
	if err != nil {
		return fmt.Errorf("could not decode pipeline hook: %w", err)
	}

	pipelineStart, err := time.Parse(GitlabTimeFormat, hook.ObjectAttributes.CreatedAt)
	if err != nil {
		return fmt.Errorf("could not parse pipeline start time: %w", err)
	}
	pipelineEnd, err := time.Parse(GitlabTimeFormat, hook.ObjectAttributes.FinishedAt)
	if err != nil {
		return fmt.Errorf("could not parse pipeline start time: %w", err)
	}

	pipelineSpan := g.tracer.StartSpan(
		"gitlab-pipeline",
		opentracing.StartTime(pipelineStart),
		opentracing.Tag{
			Key:   "instance",
			Value: instance.name,
		},
		hook,
	)

	for _, build := range hook.Builds {
		if build.FinishedAt == nil || build.StartedAt == nil {
			continue
		}

		buildStart, err := time.Parse(GitlabTimeFormat, build.StartedAt.(string))
		if err != nil {
			return fmt.Errorf("could not parse build start time: %w", err)
		}
		buildEnd, err := time.Parse(GitlabTimeFormat, build.FinishedAt.(string))
		if err != nil {
			return fmt.Errorf("could not parse build end time: %w", err)
		}

		g.tracer.StartSpan("gitlab-job",
			opentracing.StartTime(buildStart),
			opentracing.ChildOf(pipelineSpan.Context()),
			opentracing.Tag{
				Key:   "instance",
				Value: instance.name,
			},
			build,
		).FinishWithOptions(opentracing.FinishOptions{FinishTime: buildEnd})
	}

	pipelineSpan.FinishWithOptions(opentracing.FinishOptions{FinishTime: pipelineEnd})

	logger.Info("ingested pipeline %d", hook.ObjectAttributes.ID)
	return nil
}

func (p pipelineHook) Apply(o *opentracing.StartSpanOptions) {
	if o.Tags == nil {
		o.Tags = make(map[string]interface{})
	}

	o.Tags["resource.name"] = p.Project.Name
	o.Tags["status"] = p.ObjectAttributes.Status
	o.Tags["id"] = p.ObjectAttributes.ID
	o.Tags["user"] = p.User.Email

	o.Tags["project.name"] = p.Project.Name
	o.Tags["project.path"] = p.Project.PathWithNamespace
	o.Tags["project.namespace"] = p.Project.Namespace
	o.Tags["project.path"] = p.Project.WebURL

	o.Tags["ref.name"] = p.ObjectAttributes.Ref
	o.Tags["ref.sha"] = p.ObjectAttributes.Sha

	// TODO: defense check
	o.Tags["mr.id"] = p.MergeRequest.ID
	o.Tags["mr.title"] = p.MergeRequest.Title
	o.Tags["mr.url"] = p.MergeRequest.URL
}

func (b pipelineBuild) Apply(o *opentracing.StartSpanOptions) {
	if o.Tags == nil {
		o.Tags = make(map[string]interface{})
	}

	o.Tags["resource.name"] = b.Name
	o.Tags["stage"] = b.Stage
	o.Tags["status"] = b.Status

	o.Tags["runner.id"] = b.Runner.ID
	o.Tags["runner.name"] = b.Runner.Description
}
