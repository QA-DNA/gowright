package browser

import (
	"encoding/json"
	"fmt"
	"time"
)

type Clock struct {
	page      *Page
	installed bool
}

func (c *Clock) Install(opts ...ClockInstallOptions) error {
	if c.installed {
		return fmt.Errorf("clock already installed")
	}
	now := "Date.now()"
	if len(opts) > 0 && !opts[0].Time.IsZero() {
		now = fmt.Sprintf("%d", opts[0].Time.UnixMilli())
	}
	script := fmt.Sprintf(`(() => {
const _origDate = Date;
const _origSetTimeout = setTimeout;
const _origClearTimeout = clearTimeout;
const _origSetInterval = setInterval;
const _origClearInterval = clearInterval;
let _now = %s;
let _timers = [];
let _nextId = 1;
let _paused = false;
window.__clockNow = () => _now;
window.__clockSetNow = (t) => { _now = t; };
window.__clockAdvance = (ms) => {
	const target = _now + ms;
	while (_timers.length > 0 && _timers[0].time <= target) {
		const t = _timers.shift();
		_now = t.time;
		try { t.fn(); } catch(e) {}
		if (t.interval > 0) {
			t.time = _now + t.interval;
			_timers.push(t);
			_timers.sort((a,b) => a.time - b.time);
		}
	}
	_now = target;
};
window.__clockPause = () => { _paused = true; };
window.__clockResume = () => { _paused = false; };
window.Date = class extends _origDate {
	constructor(...args) {
		if (args.length === 0) { super(_now); } else { super(...args); }
	}
	static now() { return _now; }
};
window.Date.parse = _origDate.parse;
window.Date.UTC = _origDate.UTC;
window.setTimeout = (fn, delay, ...args) => {
	const id = _nextId++;
	_timers.push({id, fn: () => fn(...args), time: _now + (delay||0), interval: 0});
	_timers.sort((a,b) => a.time - b.time);
	return id;
};
window.clearTimeout = (id) => {
	_timers = _timers.filter(t => t.id !== id);
};
window.setInterval = (fn, delay, ...args) => {
	const id = _nextId++;
	_timers.push({id, fn: () => fn(...args), time: _now + (delay||0), interval: delay||0});
	_timers.sort((a,b) => a.time - b.time);
	return id;
};
window.clearInterval = window.clearTimeout;
window.performance.now = () => _now;
})()`, now)
	_, err := c.page.Evaluate(script)
	if err != nil {
		return fmt.Errorf("install clock: %w", err)
	}
	c.installed = true
	return nil
}

type ClockInstallOptions struct {
	Time time.Time
}

func (c *Clock) FastForward(d time.Duration) error {
	if !c.installed {
		return fmt.Errorf("clock not installed")
	}
	_, err := c.page.Evaluate(fmt.Sprintf("window.__clockAdvance(%d)", d.Milliseconds()))
	return err
}

func (c *Clock) PauseAt(t time.Time) error {
	if !c.installed {
		return fmt.Errorf("clock not installed")
	}
	_, err := c.page.Evaluate(fmt.Sprintf("window.__clockSetNow(%d); window.__clockPause()", t.UnixMilli()))
	return err
}

func (c *Clock) Resume() error {
	if !c.installed {
		return fmt.Errorf("clock not installed")
	}
	_, err := c.page.Evaluate("window.__clockResume()")
	return err
}

func (c *Clock) RunFor(d time.Duration) error {
	return c.FastForward(d)
}

func (c *Clock) SetFixedTime(t time.Time) error {
	if !c.installed {
		return fmt.Errorf("clock not installed")
	}
	_, err := c.page.Evaluate(fmt.Sprintf("window.__clockSetNow(%d); window.__clockPause()", t.UnixMilli()))
	return err
}

func (c *Clock) SetSystemTime(t time.Time) error {
	if !c.installed {
		return fmt.Errorf("clock not installed")
	}
	_, err := c.page.Evaluate(fmt.Sprintf("window.__clockSetNow(%d)", t.UnixMilli()))
	return err
}

func (c *Clock) Now() (time.Time, error) {
	if !c.installed {
		return time.Time{}, fmt.Errorf("clock not installed")
	}
	val, err := c.page.Evaluate("window.__clockNow()")
	if err != nil {
		return time.Time{}, err
	}
	var ms float64
	json.Unmarshal(val, &ms)
	return time.UnixMilli(int64(ms)), nil
}
