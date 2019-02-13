package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strconv"
	"time"
	"regexp"
)

type ProjectStats struct {
	CommitCount      int `json:"commit_count"`
	StorageSize      int `json:"storage_size"`
	RepositorySize   int `json:"repository_size"`
	LfsObjectSize    int `json:"lfs_object_size"`
	JobArtifactsSize int `json:"job_artifacts_size"`
}

type Project struct {
	Id								int					 `json:"id"`
	PathWithNamespace string       `json:"path_with_namespace"`
	StarCount         int          `json:"star_count"`
	ForkCount         int          `json:"fork_count"`
	OpenIssueCount    int          `json:"open_issues_count"`
	LastActivityAt    time.Time    `json:"last_activity_at"`
	Statistics        ProjectStats `json:"statistics"`
	MergeRequests			[]MergeRequest
}

type MergeRequest struct {
	Title 				string   		 `json:"title"`
	State 				string   		 `json:"state"`
	MergeStatus   string   		 `json:"merge_status"`
	TargetBranch  string   		 `json:"target_branch"`
	CreatedAt    	time.Time    `json:"created_at"`
	UpdatedAt    	time.Time    `json:"updated_at"`
}

func (project *Project) SetMergeRequests(mergeRequests []MergeRequest) {
	project.MergeRequests = mergeRequests
}


/**
 *	Extracts the stats from a single project into a
 *	prometheus compatible string.
 *
 *	@param project The project to extract stats from
 *	@return A prometheus style statistics string for the project
 */
func (project Project) PrometheusStats() string {
	path := regexp.MustCompile(`\/`).ReplaceAllString(project.PathWithNamespace, "___")
	stats := ""
	stats = fmt.Sprintf("%s\ngitlab_project_stars{repo=\"%s\"} %d", stats, path, project.StarCount)
	stats = fmt.Sprintf("%s\ngitlab_project_forks{repo=\"%s\"} %d", stats, path, project.ForkCount)
	stats = fmt.Sprintf("%s\ngitlab_project_commit_count{repo=\"%s\"} %d", stats, path, project.Statistics.CommitCount)
	stats = fmt.Sprintf("%s\ngitlab_project_storage_size{repo=\"%s\"} %d", stats, path, project.Statistics.StorageSize)
	stats = fmt.Sprintf("%s\ngitlab_project_repository_size{repo=\"%s\"} %d", stats, path, project.Statistics.RepositorySize)
	stats = fmt.Sprintf("%s\ngitlab_project_lfs_object_size{repo=\"%s\"} %d", stats, path, project.Statistics.LfsObjectSize)
	stats = fmt.Sprintf("%s\ngitlab_project_job_artifacts_size{repo=\"%s\"} %d", stats, path, project.Statistics.JobArtifactsSize)

	for _, mergeRequest := range project.MergeRequests {
		stats = fmt.Sprintf("%s\ngitlab_project_merge_request{repo=\"%s\", state=\"%s\", merge_status=\"%s\", target_branch=\"%s\"} 1", stats, path, mergeRequest.State, mergeRequest.MergeStatus, mergeRequest.TargetBranch)
	}

	stats = fmt.Sprintf("%s\ngitlab_project_last_activity{repo=\"%s\"} %d", stats, path, project.LastActivityAt.Unix())
	return stats
}

/**
 *	Fetches all merge requests from a specific project id.
 *
 *	@return A list of all merge requests known to the current gitlab project
 */
 func (project Project) GetMergeRequests(gitlabUrl string, token string) []MergeRequest {
	merge_requests := make([]MergeRequest, 0)
	page := 1
	
	for true {
		// Fetch a page from the API.
		mergesUrl := fmt.Sprintf("%s/api/v4/projects/%d/merge_requests?private_token=%s&per_page=100&statistics=1&page=%d", gitlabUrl, project.Id, token, page)
		log.Printf("Requesting %s\n", mergesUrl)
		response, error := http.Get(mergesUrl)
		if error != nil {
			log.Fatalf(error.Error())
		}

		// Merge the results back to the complete array.
		mergesInPage := make([]MergeRequest, 0)
		error = json.NewDecoder(response.Body).Decode(&mergesInPage)
		if error != nil {
			log.Fatalf(error.Error())
		}
		merge_requests = append(merge_requests, mergesInPage...)

		// Parse the X-Next-Page Header in order to figure out
		// if another page should be requested.
		// If not, then return the current merge requests.
		pageHeader := response.Header["X-Next-Page"][0]
		if pageHeader == "" {
			return merge_requests
		}
		page, error = strconv.Atoi(pageHeader)
		if error != nil {
			log.Fatalf(error.Error())
		}
	}

	return merge_requests
}


/**
 *	Fetches all projects from the configured gitlab endpoint.
 *
 *	@return A list of all projects known to the current gitlab instance
 */
func GetRepositories(gitlabUrl string, token string) []Project {
	projects := make([]Project, 0)
	page := 1

	for true {
		// Fetch a page from the API.
		projectsUrl := fmt.Sprintf("%s/api/v4/projects?private_token=%s&per_page=100&statistics=1&page=%d", gitlabUrl, token, page)
		log.Printf("Requesting %s\n", projectsUrl)
		response, error := http.Get(projectsUrl)
		if error != nil {
			log.Fatalf(error.Error())
		}

		// Merge the results back to the complete array.
		projectsInPage := make([]Project, 0)
		error = json.NewDecoder(response.Body).Decode(&projectsInPage)
		if error != nil {
			log.Fatalf(error.Error())
		}

		for i := range projectsInPage {
			merge_requests := projectsInPage[i].GetMergeRequests(gitlabUrl, token)
			projectsInPage[i].SetMergeRequests(merge_requests)
		}
		
		projects = append(projects, projectsInPage...)		
		// Parse the X-Next-Page Header in order to figure out
		// if another page should be requested.
		// If not, then return the current projects.
		pageHeader := response.Header["X-Next-Page"][0]
		if pageHeader == "" {
			return projects
		}
		page, error = strconv.Atoi(pageHeader)
		if error != nil {
			log.Fatalf(error.Error())
		}
	}

	return projects
}
