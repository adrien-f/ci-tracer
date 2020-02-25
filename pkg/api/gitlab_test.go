package api

import (
	"testing"

	"github.com/opentracing/opentracing-go/mocktracer"
	"github.com/sirupsen/logrus/hooks/test"
	"github.com/stretchr/testify/assert"
)

type testSpan struct {
	operationName string
	tags          map[string]interface{}
}

func TestProcessBuild(t *testing.T) {
	logger, _ := test.NewNullLogger()
	tracer := mocktracer.New()
	api := NewGitlabAPI(logger, tracer)
	api.instances["test"] = &GitlabInstance{
		name: "test",
	}
	for _, tcase := range []struct {
		build *pipelineBuild
		spans []testSpan
	}{
		{
			build: &pipelineBuild{
				ID:         1,
				Stage:      "build",
				Name:       "build",
				Status:     "success",
				CreatedAt:  "2020-02-15 15:23:28 UTC",
				StartedAt:  "2020-02-15 15:26:12 UTC",
				FinishedAt: "2020-02-15 15:26:29 UTC",
				Runner: runner{
					ID:          1,
					Description: "runner.gitlab.com",
				},
			},
			spans: []testSpan{
				{
					operationName: "gitlab-job",
					tags: map[string]interface{}{
						"instance":      "test",
						"resource.name": "build",
						"stage":         "build",
						"status":        "success",
						"runner.id":     1,
						"runner.name":   "runner.gitlab.com",
					},
				},
			},
		},
	} {
		spanCtx := tracer.StartSpan("pipeline").Context()
		api.processBuild(tcase.build, spanCtx, api.instances["test"])
		assert.Len(t, tracer.FinishedSpans(), 1)
		for index, span := range tracer.FinishedSpans() {
			assert.Equal(t, tcase.spans[index].operationName, span.OperationName)
			tags := span.Tags()
			assert.Len(t, tags, len(tcase.spans[index].tags))
			for key, value := range tcase.spans[index].tags {
				assert.Equal(t, value, tags[key])
			}
		}
		tracer.Reset()
	}
}
