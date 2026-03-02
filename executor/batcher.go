package executor

import (
	"errors"
	"fmt"

	"ohakan-mcp/planner"
)

// ExecutionBatch aynı anda (paralel) çalışabilecek görev grubunu temsil eder.
type ExecutionBatch struct {
	BatchIndex int        `json:"batch_index"`
	Tasks      []ExecTask `json:"tasks"`
}

// ExecTask bir DAGTask'ın yürütme sırasındaki zenginleştirilmiş halidir.
type ExecTask struct {
	ID             string   `json:"id"`
	Name           string   `json:"name"`
	Action         string   `json:"action"`
	Files          []string `json:"files"`
	DependsOn      []string `json:"depends_on"`
	SubAgentPrompt string   `json:"sub_agent_prompt"`
}

// BatchDAG bir DAG'ı topological sort (Kahn algoritması) ile
// sıralı yürütme batch'lerine böler.
// Aynı batch içindeki görevler bağımsızdır ve paralel çalıştırılabilir.
// DAG döngü içeriyorsa hata döner.
func BatchDAG(dag *planner.DAG) ([]ExecutionBatch, error) {
	if len(dag.Tasks) == 0 {
		return nil, errors.New("DAG boş")
	}

	// id → task indeksi haritası
	idx := make(map[string]int, len(dag.Tasks))
	for i, t := range dag.Tasks {
		if _, dup := idx[t.ID]; dup {
			return nil, fmt.Errorf("DAG'da tekrar eden görev ID: %q", t.ID)
		}
		idx[t.ID] = i
	}

	// Bağımlılık doğrulaması
	for _, t := range dag.Tasks {
		for _, dep := range t.DependsOn {
			if _, ok := idx[dep]; !ok {
				return nil, fmt.Errorf("görev %q bilinmeyen bağımlılığa referans veriyor: %q", t.ID, dep)
			}
		}
	}

	// in-degree hesapla (kaç göreve bağımlı)
	inDegree := make(map[string]int, len(dag.Tasks))
	// reverse adjacency: dep → ondan bekleyenler
	dependents := make(map[string][]string, len(dag.Tasks))
	for _, t := range dag.Tasks {
		if _, exists := inDegree[t.ID]; !exists {
			inDegree[t.ID] = 0
		}
		for _, dep := range t.DependsOn {
			inDegree[t.ID]++
			dependents[dep] = append(dependents[dep], t.ID)
		}
	}

	// Kahn algoritması: her iterasyonda in-degree=0 olanlar bir batch oluşturur
	var batches []ExecutionBatch
	processed := 0

	for {
		// Bu iterasyonda kuyruğa girecekler
		var ready []string
		for id, deg := range inDegree {
			if deg == 0 {
				ready = append(ready, id)
			}
		}
		if len(ready) == 0 {
			break
		}

		// Tekrarlanabilir sıra için sırala
		sortStrings(ready)

		batch := ExecutionBatch{BatchIndex: len(batches)}
		for _, id := range ready {
			t := dag.Tasks[idx[id]]
			batch.Tasks = append(batch.Tasks, ExecTask{
				ID:        t.ID,
				Name:      t.Name,
				Action:    t.Action,
				Files:     t.Files,
				DependsOn: t.DependsOn,
			})
			delete(inDegree, id)
			processed++
		}
		batches = append(batches, batch)

		// Tamamlanan görevlerin bağımlılarının in-degree'sini düşür
		for _, id := range ready {
			for _, dependent := range dependents[id] {
				inDegree[dependent]--
			}
		}
	}

	// Döngü tespiti: tüm görevler işlenemediyse döngü vardır
	if processed != len(dag.Tasks) {
		return nil, errors.New("DAG döngü içeriyor — topological sort tamamlanamadı")
	}

	return batches, nil
}

// sortStrings küçük diziler için yerinde insertion sort (stdlib import eklememek için).
func sortStrings(s []string) {
	for i := 1; i < len(s); i++ {
		key := s[i]
		j := i - 1
		for j >= 0 && s[j] > key {
			s[j+1] = s[j]
			j--
		}
		s[j+1] = key
	}
}
