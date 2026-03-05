package browser

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/PeterStoica/gowright/pkg/cdp"
)

func checkVisible(ctx context.Context, session *cdp.Session, objectID string) error {
	visible, err := isElementVisible(ctx, session, objectID)
	if err != nil {
		return err
	}
	if !visible {
		return fmt.Errorf("element is not visible")
	}
	return nil
}

func checkEnabled(ctx context.Context, session *cdp.Session, objectID string) error {
	enabled, err := isElementEnabled(ctx, session, objectID)
	if err != nil {
		return err
	}
	if !enabled {
		return fmt.Errorf("element is disabled")
	}
	return nil
}

func checkStable(ctx context.Context, session *cdp.Session, objectID string) error {
	raw, err := callFunctionOn(ctx, session, objectID, `function() {
		var el = this;
		var r1 = el.getBoundingClientRect();
		return new Promise(function(resolve) {
			setTimeout(function() {
				var r2 = el.getBoundingClientRect();
				resolve(r1.x === r2.x && r1.y === r2.y && r1.width === r2.width && r1.height === r2.height);
			}, 15);
		});
	}`, "")
	if err != nil {
		return err
	}
	var stable bool
	json.Unmarshal(raw, &stable)
	if !stable {
		return fmt.Errorf("element is not stable")
	}
	return nil
}

func checkHitTarget(ctx context.Context, session *cdp.Session, objectID string) error {
	raw, err := callFunctionOn(ctx, session, objectID, `function() {
		var rect = this.getBoundingClientRect();
		var cx = rect.left + rect.width / 2;
		var cy = rect.top + rect.height / 2;
		var hit = document.elementFromPoint(cx, cy);
		if (!hit) return false;
		return this.contains(hit) || hit.contains(this) || this === hit;
	}`, "")
	if err != nil {
		return err
	}
	var ok bool
	json.Unmarshal(raw, &ok)
	if !ok {
		return fmt.Errorf("element is obscured by another element")
	}
	return nil
}

func checkEditable(ctx context.Context, session *cdp.Session, objectID string) error {
	raw, err := callFunctionOn(ctx, session, objectID, `function() { return !this.readOnly && !this.disabled; }`, "")
	if err != nil {
		return err
	}
	var editable bool
	json.Unmarshal(raw, &editable)
	if !editable {
		return fmt.Errorf("element is not editable")
	}
	return nil
}
