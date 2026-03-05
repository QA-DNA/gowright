package gowright_test

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/QA-DNA/gowright"
	"github.com/QA-DNA/gowright/pkg/expect"
)

func TestCreateWorkOrder(t *testing.T) {
	baseURL := os.Getenv("BASE_URL")
	if baseURL == "" {
		baseURL = "https://modern.pitstopconnect.com"
	}
	email := os.Getenv("TEST_EMAIL")
	password := os.Getenv("TEST_PASSWORD")
	if email == "" || password == "" {
		t.Skip("TEST_EMAIL and TEST_PASSWORD env vars required")
	}

	const timHortonsFleet = "Tim Horton's - (QA)"
	const spinner = ".ant-spin-spinning"

	ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
	t.Cleanup(cancel)

	b, err := gowright.Launch(ctx, gowright.LaunchOptions{
		Headless: false,
		Args:     []string{"--window-size=1920,1080"},
	})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { b.Close() })

	page, err := b.NewPage()
	if err != nil {
		t.Fatal(err)
	}

	if err := page.SetViewportSize(1920, 1080); err != nil {
		t.Fatal(err)
	}

	waitForLoading := func() {
		page.Locator(spinner).First().WaitFor(gowright.WaitForOptions{
			State:   "hidden",
			Timeout: 30 * time.Second,
		})
	}

	// selectOptionFromDropdown — matches Playwright's selectOptionFromDropdown exactly.
	// Playwright uses xpath=ancestor to find the .ant-select-selector ancestor,
	// then clicks it with real mouse events. We use CSS :has() to achieve the same.
	selectOptionFromDropdown := func(inputID string, index int) {
		// Click the .ant-select-selector that contains the input — real mouse click via CDP
		selectorCSS := fmt.Sprintf(".ant-select:has(%s) .ant-select-selector", inputID)
		if err := page.Locator(selectorCSS).Click(); err != nil {
			t.Fatal("click dropdown "+inputID+":", err)
		}

		dropdown := page.Locator("div.ant-select-dropdown:visible").First()
		if err := dropdown.WaitFor(gowright.WaitForOptions{State: "visible", Timeout: 10 * time.Second}); err != nil {
			t.Fatal("wait for dropdown visible:", err)
		}
		time.Sleep(300 * time.Millisecond)

		// Click the option with real mouse events — Ant Design needs proper mouse events
		// to trigger its onChange handlers. Using the parent option element (not just content).
		option := page.Locator("div.ant-select-dropdown:visible div.ant-select-item-option").Nth(index)
		if err := option.WaitFor(gowright.WaitForOptions{State: "visible", Timeout: 10 * time.Second}); err != nil {
			t.Fatal("wait for option visible:", err)
		}
		if err := option.Click(); err != nil {
			t.Fatal("click option:", err)
		}
		time.Sleep(300 * time.Millisecond)

		// Wait for dropdown to close (swallow error like Playwright's .catch(() => {}))
		page.Locator("div.ant-select-dropdown:visible").WaitFor(gowright.WaitForOptions{
			State:   "detached",
			Timeout: 2 * time.Second,
		})
	}

	// --- Login ---
	// Matches: await wo.login()
	t.Log("Step: Login")

	if err := page.Goto(baseURL + "/login"); err != nil {
		t.Fatal(err)
	}
	if err := page.GetByTestId("login-email-input").Fill(email); err != nil {
		t.Fatal("fill email:", err)
	}
	if err := page.GetByTestId("login-password-input").Fill(password); err != nil {
		t.Fatal("fill password:", err)
	}
	if err := page.GetByTestId("login-button").Click(); err != nil {
		t.Fatal("click login:", err)
	}
	if err := page.WaitForURL("**/vehicles**"); err != nil {
		t.Fatal(err)
	}
	waitForLoading()
	t.Log("Step: Login complete")

	// THEN: await expect(page).toHaveURL(/\/vehicles/)
	if err := expect.Page(page).WithTimeout(10 * time.Second).ToHaveURL("/vehicles"); err != nil {
		t.Fatal(err)
	}

	// --- Navigate to Work Order Management ---
	// Matches: await wo.selectMenuByText("Work Order Management")
	// Playwright: this.menuButton.click() then page.locator("span.ant-menu-title-content", {hasText: menuText}).click()
	t.Log("Step: Navigate to Work Order Management")

	if err := page.GetByTestId("header-menu-button").Click(); err != nil {
		t.Fatal("click menu button:", err)
	}
	if err := page.Locator("span.ant-menu-title-content").Filter(gowright.FilterOptions{
		HasText: "Work Order Management",
	}).Click(); err != nil {
		t.Fatal("click Work Order menu:", err)
	}
	waitForLoading()

	// THEN: await expect(page).toHaveURL(/\/work-order/)
	if err := expect.Page(page).WithTimeout(15 * time.Second).ToHaveURL("/work-order"); err != nil {
		t.Fatal(err)
	}
	t.Log("Step: On work order page")

	// --- Select fleet ---
	// Matches: await wo.selectFleet(TIM_HORTONS_FLEET)
	// Playwright: fleetDropdown.click() → fleetDropdown.locator("input").fill(name) → click option
	t.Log("Step: Select fleet")

	fleetDropdown := page.GetByTestId("fleet-selector")
	if err := fleetDropdown.Click(); err != nil {
		t.Fatal("click fleet dropdown:", err)
	}
	if err := fleetDropdown.Locator("input").Fill(timHortonsFleet); err != nil {
		t.Fatal("fill fleet search:", err)
	}
	if err := page.Locator(`.ant-select-item-option[title="` + timHortonsFleet + `"]`).Click(); err != nil {
		t.Fatal("click fleet option:", err)
	}
	waitForLoading()

	// THEN: await expect(wo.fleetDropdown).toContainText(TIM_HORTONS_FLEET)
	if err := expect.Locator(fleetDropdown).WithTimeout(10 * time.Second).ToContainText(timHortonsFleet); err != nil {
		t.Fatal(err)
	}
	t.Log("Step: Fleet selected")

	// --- Create Work Order ---
	// Matches: await wo.addNewWorkOrderButton.click()
	t.Log("Step: Create work order")

	// Set up response listener BEFORE clicking — the API call fires immediately on slider open
	invoiceCh := make(chan *gowright.Response, 1)
	page.On("response", func(v any) {
		if resp, ok := v.(*gowright.Response); ok {
			if resp.Request() != nil && resp.Request().Method == "GET" &&
				strings.Contains(resp.URL, "/v1/work-order/unique-invoice-number") {
				select {
				case invoiceCh <- resp:
				default:
				}
			}
		}
	})

	if err := page.GetByTestId("add-new-work-order-button").Click(); err != nil {
		t.Fatal("click add work order:", err)
	}

	// THEN: await expect(wo.createWorkOrderSlider).toBeVisible()
	if err := expect.Locator(page.GetByTestId("new-work-order-slider")).WithTimeout(10 * time.Second).ToBeVisible(); err != nil {
		t.Fatal(err)
	}

	// Wait for the unique-invoice-number API response
	select {
	case invoiceResp := <-invoiceCh:
		t.Logf("Step: Got invoice number response (status=%d, url=%s)", invoiceResp.Status, invoiceResp.URL)
	case <-time.After(15 * time.Second):
		t.Fatal("timed out waiting for unique-invoice-number response")
	}

	// Matches: await wo.openFullPageButton.click()
	if err := page.GetByTestId("open-full-page-button").Click(); err != nil {
		t.Fatal("click open full page:", err)
	}
	waitForLoading()

	// THEN: await expect(page).toHaveURL(/\/work-order\/add/)
	if err := expect.Page(page).WithTimeout(10 * time.Second).ToHaveURL("/work-order/add"); err != nil {
		t.Fatal(err)
	}
	t.Log("Step: On work order form page")

	// Wait for WO Number field to be populated (React needs time to render API response)
	time.Sleep(2 * time.Second)

	// --- Fill Work Order Form ---

	// Matches: await wo.selectOptionFromDropdown(wo.assignedToDropdown, 0)
	// Playwright uses: page.locator("#woAssignedTo").locator("xpath=ancestor::div[contains(@class, 'ant-select-selector')]")
	t.Log("Step: Select Assigned To")
	selectOptionFromDropdown("#woAssignedTo", 0)
	t.Log("Step: Assigned To selected")

	// Matches: await wo.selectOptionFromDropdown(wo.selectAssetDropdown, 0)
	t.Log("Step: Select Asset")
	selectOptionFromDropdown("#woSelectVehicle", 0)
	t.Log("Step: Asset selected")

	// Matches: await wo.selectRepairType("Driver Identified")
	t.Log("Step: Select repair type")
	if err := page.Locator("#woRepairType").Locator("label.ant-radio-wrapper").Filter(gowright.FilterOptions{
		HasText: "Driver Identified",
	}).Click(); err != nil {
		t.Fatal("click repair type:", err)
	}
	t.Log("Step: Repair type selected")

	// --- Debug: check WO Number field value ---
	woNumVal, err := page.Evaluate(`document.querySelector('#woNumber input, #woNumber, input[id*="woNumber"]')?.value || 'NOT FOUND'`)
	if err != nil {
		t.Log("Step: Could not read WO Number field:", err)
	} else {
		t.Logf("Step: WO Number field value: %s", string(woNumVal))
	}

	// --- Debug: take screenshot before save ---
	screenshot, _ := page.Screenshot()
	if len(screenshot) > 0 {
		os.WriteFile("/tmp/gowright-before-save.png", screenshot, 0o644)
		t.Log("Step: Screenshot saved to /tmp/gowright-before-save.png")
	}

	// --- Save & Exit ---
	// Matches Playwright: Promise.all([page.waitForResponse(...), wo.saveExitButton.click()])
	t.Log("Step: Save & Exit")

	// Debug: log all responses to see what's happening
	page.On("response", func(v any) {
		if resp, ok := v.(*gowright.Response); ok {
			method := "?"
			if resp.Request() != nil {
				method = resp.Request().Method
			}
			t.Logf("  [response] %s %s (status=%d)", method, resp.URL, resp.Status)
		}
	})

	respCh := make(chan *gowright.Response, 1)
	page.On("response", func(v any) {
		resp, ok := v.(*gowright.Response)
		if !ok {
			return
		}
		if resp.Request() != nil && resp.Request().Method == "POST" {
			select {
			case respCh <- resp:
			default:
			}
		}
	})

	// Dismiss any help widget overlay that might be blocking the button
	page.Evaluate(`(function() { var el = document.querySelector('.intercom-lightweight-app, [class*="freshchat"], [class*="help-widget"], iframe[title*="chat"]'); if (el) el.style.display = 'none'; })()`)

	if err := page.GetByTestId("work-order-save-exit-button").Click(); err != nil {
		t.Fatal("click save:", err)
	}

	// Also take screenshot after save click
	screenshot2, _ := page.Screenshot()
	if len(screenshot2) > 0 {
		os.WriteFile("/tmp/gowright-after-save.png", screenshot2, 0o644)
		t.Log("Step: Post-save screenshot at /tmp/gowright-after-save.png")
	}

	var createdResp *gowright.Response
	select {
	case createdResp = <-respCh:
	case <-time.After(30 * time.Second):
		t.Fatal("timed out waiting for work order POST response")
	}

	body, err := createdResp.Json()
	if err != nil {
		t.Fatal(err)
	}

	var createdWorkOrder struct {
		ID            string `json:"id"`
		InvoiceNumber string `json:"invoice_number"`
	}
	if err := json.Unmarshal(body, &createdWorkOrder); err != nil {
		t.Fatal(err)
	}
	if createdWorkOrder.ID == "" {
		t.Fatal("expected work order to have an ID")
	}

	workOrderNumber := createdWorkOrder.InvoiceNumber
	t.Log("Step: Work order created:", workOrderNumber)

	// --- Verify work order in table ---
	// Matches: await expect(page).toHaveURL(/\/work-order/)
	if err := expect.Page(page).WithTimeout(15 * time.Second).ToHaveURL("/work-order"); err != nil {
		t.Fatal(err)
	}
	waitForLoading()

	// Matches: await wo.searchForWorkOrder(workOrderNumber)
	t.Log("Step: Search for work order")
	searchInput := page.GetByTestId("work-order-search-input")
	if err := searchInput.Click(); err != nil {
		t.Fatal("click search:", err)
	}
	if err := searchInput.Fill(workOrderNumber); err != nil {
		t.Fatal("fill search:", err)
	}
	if err := searchInput.Press("Enter"); err != nil {
		t.Fatal("press enter:", err)
	}
	waitForLoading()

	// Matches: await wo.assertWorkOrderInTable(workOrderNumber)
	row := page.GetByTestId("work-order-row").Filter(gowright.FilterOptions{
		HasText: workOrderNumber,
	}).First()
	if err := expect.Locator(row).WithTimeout(10 * time.Second).ToBeVisible(); err != nil {
		t.Fatal("work order not found in table: " + err.Error())
	}
	t.Log("Step: Work order verified in table")
}
