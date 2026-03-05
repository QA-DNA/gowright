package browser

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/QA-DNA/gowright/pkg/cdp"
)

type Accessibility struct {
	page *Page
}

type AccessibilityNode struct {
	Role        string              `json:"role"`
	Name        string              `json:"name"`
	Value       string              `json:"value"`
	Description string              `json:"description"`
	Focused     bool                `json:"focused"`
	Selected    bool                `json:"selected"`
	Disabled    bool                `json:"disabled"`
	Expanded    *bool               `json:"expanded,omitempty"`
	Checked     *bool               `json:"checked,omitempty"`
	Level       int                 `json:"level,omitempty"`
	Children    []AccessibilityNode `json:"children,omitempty"`
}

type AccessibilitySnapshotOptions struct {
	Root            *Locator
	InterestingOnly bool
}

func (a *Accessibility) Snapshot(opts ...AccessibilitySnapshotOptions) (*AccessibilityNode, error) {
	ctx := a.page.browser.ctx

	params := map[string]any{}
	if len(opts) > 0 && opts[0].Root != nil {
		objectID, err := evaluateHandle(ctx, a.page.session, opts[0].Root.selector)
		if err != nil {
			return nil, err
		}
		if objectID != "" {
			backendNodeID, err := getBackendNodeID(ctx, a.page.session, objectID)
			if err == nil && backendNodeID > 0 {
				params["backendNodeId"] = backendNodeID
			}
			releaseObject(ctx, a.page.session, objectID)
		}
	}

	result, err := a.page.session.Call(ctx, "Accessibility.getFullAXTree", params)
	if err != nil {
		return nil, fmt.Errorf("get accessibility tree: %w", err)
	}

	var resp struct {
		Nodes []axNode `json:"nodes"`
	}
	if err := json.Unmarshal(result, &resp); err != nil {
		return nil, err
	}

	if len(resp.Nodes) == 0 {
		return nil, nil
	}

	interestingOnly := true
	if len(opts) > 0 {
		interestingOnly = opts[0].InterestingOnly
	}

	nodeMap := make(map[string]*axNode)
	for i := range resp.Nodes {
		nodeMap[resp.Nodes[i].NodeID] = &resp.Nodes[i]
	}

	root := buildAccessibilityTree(nodeMap, resp.Nodes[0].NodeID, interestingOnly)
	return root, nil
}

type axNode struct {
	NodeID      string       `json:"nodeId"`
	ParentID    string       `json:"parentId"`
	Role        axValue      `json:"role"`
	Name        axValue      `json:"name"`
	Value       axValue      `json:"value"`
	Description axValue      `json:"description"`
	Properties  []axProperty `json:"properties"`
	ChildIDs    []string     `json:"childIds"`
}

type axValue struct {
	Type  string          `json:"type"`
	Value json.RawMessage `json:"value"`
}

func (v axValue) String() string {
	var s string
	if json.Unmarshal(v.Value, &s) == nil {
		return s
	}
	return string(v.Value)
}

type axProperty struct {
	Name  string  `json:"name"`
	Value axValue `json:"value"`
}

func getBackendNodeID(ctx context.Context, session *cdp.Session, objectID string) (int, error) {
	result, err := session.Call(ctx, "DOM.describeNode", map[string]any{
		"objectId": objectID,
	})
	if err != nil {
		return 0, err
	}
	var resp struct {
		Node struct {
			BackendNodeID int `json:"backendNodeId"`
		} `json:"node"`
	}
	json.Unmarshal(result, &resp)
	return resp.Node.BackendNodeID, nil
}

func buildAccessibilityTree(nodeMap map[string]*axNode, nodeID string, interestingOnly bool) *AccessibilityNode {
	n, ok := nodeMap[nodeID]
	if !ok {
		return nil
	}

	role := n.Role.String()
	name := n.Name.String()

	if interestingOnly && isBoringNode(role, name) {
		var firstInteresting *AccessibilityNode
		for _, childID := range n.ChildIDs {
			child := buildAccessibilityTree(nodeMap, childID, interestingOnly)
			if child != nil {
				if firstInteresting == nil {
					firstInteresting = child
				}
			}
		}
		return firstInteresting
	}

	node := &AccessibilityNode{
		Role:        role,
		Name:        name,
		Value:       n.Value.String(),
		Description: n.Description.String(),
	}

	for _, prop := range n.Properties {
		var bval bool
		switch prop.Name {
		case "focused":
			json.Unmarshal(prop.Value.Value, &bval)
			node.Focused = bval
		case "selected":
			json.Unmarshal(prop.Value.Value, &bval)
			node.Selected = bval
		case "disabled":
			json.Unmarshal(prop.Value.Value, &bval)
			node.Disabled = bval
		case "expanded":
			json.Unmarshal(prop.Value.Value, &bval)
			node.Expanded = &bval
		case "checked":
			json.Unmarshal(prop.Value.Value, &bval)
			node.Checked = &bval
		case "level":
			var ival int
			json.Unmarshal(prop.Value.Value, &ival)
			node.Level = ival
		}
	}

	for _, childID := range n.ChildIDs {
		child := buildAccessibilityTree(nodeMap, childID, interestingOnly)
		if child != nil {
			node.Children = append(node.Children, *child)
		}
	}

	return node
}

func isBoringNode(role, name string) bool {
	switch role {
	case "none", "generic", "InlineTextBox", "StaticText", "LineBreak":
		return name == ""
	}
	return false
}
