package indexer

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/rs/zerolog"
)

type Indexer struct {
	retrieverURL string
	client       *http.Client
	log          zerolog.Logger
}

func New(retrieverURL string, log zerolog.Logger) *Indexer {
	return &Indexer{
		retrieverURL: strings.TrimRight(retrieverURL, "/"),
		client:       &http.Client{Timeout: 10 * time.Second},
		log:          log,
	}
}

type IndexPoint struct {
	Collection string         `json:"collection"`
	ID         string         `json:"id"`
	Text       string         `json:"text"`
	Metadata   map[string]any `json:"metadata"`
}

func (idx *Indexer) IndexPoint(ctx context.Context, userID string, p IndexPoint) error {
	payload, err := json.Marshal(p)
	if err != nil {
		return err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, idx.retrieverURL+"/index", bytes.NewReader(payload))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Strata-User", userID)

	resp, err := idx.client.Do(req)
	if err != nil {
		return fmt.Errorf("post index point: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("retriever index failed: HTTP %d", resp.StatusCode)
	}
	return nil
}

type PodInfo struct {
	Name      string `json:"name"`
	Namespace string `json:"namespace"`
	Node      string `json:"node"`
	Phase     string `json:"phase"`
	Ready     string `json:"ready"`
	Restarts  int    `json:"restarts"`
	Age       string `json:"age"`
}

type HistoryInfo struct {
	ActionType string `json:"action_type"`
	Target     string `json:"target"`
	Status     string `json:"status"`
	Details    string `json:"details"`
	ClientType string `json:"client_type"`
	CreatedAt  string `json:"created_at"`
}

// IndexClusterResources indexes cluster summary, workloads, and audit history.
func (idx *Indexer) IndexClusterResources(
	ctx context.Context,
	userID string,
	clusterID string,
	clusterName string,
	contextName string,
	pods []PodInfo,
	history []HistoryInfo,
) (int, error) {
	count := 0

	// 1. Cluster summary chunk
	summaryText := fmt.Sprintf(
		"Cluster %s (ID: %s, Context: %s). Total pods: %d. Status: Ready.",
		clusterName, clusterID, contextName, len(pods),
	)
	err := idx.IndexPoint(ctx, userID, IndexPoint{
		Collection: "clusters",
		ID:         fmt.Sprintf("%s/summary", clusterID),
		Text:       summaryText,
		Metadata: map[string]any{
			"cluster_id":   clusterID,
			"cluster_name": clusterName,
			"kind":         "cluster_summary",
		},
	})
	if err != nil {
		return count, err
	}
	count++

	// 2. Pod workload chunks
	for _, p := range pods {
		podText := fmt.Sprintf(
			"Cluster: %s (%s) | Namespace: %s | Pod: %s | Phase: %s | Ready: %s | Restarts: %d | Node: %s | Age: %s",
			clusterName, clusterID, p.Namespace, p.Name, p.Phase, p.Ready, p.Restarts, p.Node, p.Age,
		)
		pointID := fmt.Sprintf("%s/pods/%s/%s", clusterID, p.Namespace, p.Name)
		err := idx.IndexPoint(ctx, userID, IndexPoint{
			Collection: "clusters",
			ID:         pointID,
			Text:       podText,
			Metadata: map[string]any{
				"cluster_id":   clusterID,
				"cluster_name": clusterName,
				"namespace":    p.Namespace,
				"pod_name":     p.Name,
				"phase":        p.Phase,
				"kind":         "pod",
			},
		})
		if err != nil {
			return count, err
		}
		count++
	}

	// 3. Recent history chunks
	for i, h := range history {
		if i >= 10 {
			break
		}
		histText := fmt.Sprintf(
			"Cluster: %s (%s) | Audit Action: %s on target %s | Status: %s | Executed via %s | Details: %s | Time: %s",
			clusterName, clusterID, h.ActionType, h.Target, h.Status, h.ClientType, h.Details, h.CreatedAt,
		)
		pointID := fmt.Sprintf("%s/history/%d", clusterID, i)
		err := idx.IndexPoint(ctx, userID, IndexPoint{
			Collection: "clusters",
			ID:         pointID,
			Text:       histText,
			Metadata: map[string]any{
				"cluster_id":   clusterID,
				"cluster_name": clusterName,
				"action_type":  h.ActionType,
				"kind":         "audit_history",
			},
		})
		if err != nil {
			return count, err
		}
		count++
	}

	return count, nil
}

// IndexDocumentation chunks and indexes markdown documents for a user.
func (idx *Indexer) IndexDocumentation(ctx context.Context, userID, docsDir string) (int, error) {
	chunks, err := LoadAndChunkDocs(docsDir)
	if err != nil {
		return 0, err
	}

	count := 0
	for _, c := range chunks {
		err := idx.IndexPoint(ctx, userID, IndexPoint{
			Collection: "docs",
			ID:         c.ID,
			Text:       c.Text,
			Metadata:   c.Metadata,
		})
		if err != nil {
			idx.log.Warn().Err(err).Str("doc", c.Path).Msg("failed to index doc chunk")
			continue
		}
		count++
	}
	return count, nil
}
