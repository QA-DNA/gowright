package gowright_test

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/QA-DNA/gowright"
)

var workOrderTest = gowright.NewTest(gowright.TestConfig{
	Headless: false,
	Timeout:  120 * time.Second,
	Viewport: &gowright.Viewport{Width: 1920, Height: 1080},
})

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

	workOrderTest.Run(t, "create and verify work order", func(pw *gowright.TestContext) {
		page := pw.Page

		waitForLoading := func() {
			page.Locator(spinner).First().WaitFor(gowright.WaitForOptions{
				State:   "hidden",
				Timeout: 30 * time.Second,
			})
		}

		selectOptionFromDropdown := func(inputID string, index int) {
			selectorCSS := fmt.Sprintf(".ant-select:has(%s) .ant-select-selector", inputID)
			page.Locator(selectorCSS).Click()

			dropdown := page.Locator("div.ant-select-dropdown:visible").First()
			dropdown.WaitFor(gowright.WaitForOptions{State: "visible", Timeout: 10 * time.Second})
			time.Sleep(300 * time.Millisecond)

			option := page.Locator("div.ant-select-dropdown:visible div.ant-select-item-option").Nth(index)
			option.WaitFor(gowright.WaitForOptions{State: "visible", Timeout: 10 * time.Second})
			option.Click()
			time.Sleep(300 * time.Millisecond)

			page.Locator("div.ant-select-dropdown:visible").WaitFor(gowright.WaitForOptions{
				State:   "detached",
				Timeout: 2 * time.Second,
			})
		}

		// --- Login ---
		t.Log("Step: Login")
		page.Goto(baseURL + "/login")
		page.GetByTestId("login-email-input").Fill(email)
		page.GetByTestId("login-password-input").Fill(password)
		page.GetByTestId("login-button").Click()
		page.WaitForURL("**/vehicles**")
		waitForLoading()
		t.Log("Step: Login complete")

		pw.Expect(pw.Page).WithTimeout(10 * time.Second).ToHaveURL("/vehicles")

		// --- Navigate to Work Order Management ---
		t.Log("Step: Navigate to Work Order Management")
		page.GetByTestId("header-menu-button").Click()
		page.Locator("span.ant-menu-title-content").Filter(gowright.FilterOptions{
			HasText: "Work Order Management",
		}).Click()
		waitForLoading()

		pw.Expect(pw.Page).WithTimeout(15 * time.Second).ToHaveURL("/work-order")
		t.Log("Step: On work order page")

		// --- Select fleet ---
		t.Log("Step: Select fleet")
		fleetDropdown := page.GetByTestId("fleet-selector")
		fleetDropdown.Click()
		fleetDropdown.Locator("input").Fill(timHortonsFleet)
		page.Locator(`.ant-select-item-option[title="` + timHortonsFleet + `"]`).Click()
		waitForLoading()

		pw.Expect(fleetDropdown).WithTimeout(10 * time.Second).ToContainText(timHortonsFleet)
		t.Log("Step: Fleet selected")

		// --- Create Work Order ---
		t.Log("Step: Create work order")

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

		page.GetByTestId("add-new-work-order-button").Click()

		pw.Expect(page.GetByTestId("new-work-order-slider")).WithTimeout(10 * time.Second).ToBeVisible()

		select {
		case invoiceResp := <-invoiceCh:
			t.Logf("Step: Got invoice number response (status=%d, url=%s)", invoiceResp.Status, invoiceResp.URL)
		case <-time.After(15 * time.Second):
			t.Fatal("timed out waiting for unique-invoice-number response")
		}

		page.GetByTestId("open-full-page-button").Click()
		waitForLoading()

		pw.Expect(pw.Page).WithTimeout(10 * time.Second).ToHaveURL("/work-order/add")
		t.Log("Step: On work order form page")

		time.Sleep(2 * time.Second)

		// --- Fill Work Order Form ---
		t.Log("Step: Select Assigned To")
		selectOptionFromDropdown("#woAssignedTo", 0)
		t.Log("Step: Assigned To selected")

		t.Log("Step: Select Asset")
		selectOptionFromDropdown("#woSelectVehicle", 0)
		t.Log("Step: Asset selected")

		t.Log("Step: Select repair type")
		page.Locator("#woRepairType").Locator("label.ant-radio-wrapper").Filter(gowright.FilterOptions{
			HasText: "Driver Identified",
		}).Click()
		t.Log("Step: Repair type selected")

		// Debug: check WO Number field value
		woNumVal := page.Evaluate(`document.querySelector('#woNumber input, #woNumber, input[id*="woNumber"]')?.value || 'NOT FOUND'`)
		t.Logf("Step: WO Number field value: %s", string(woNumVal))

		// Debug: screenshot before save
		screenshot := page.Screenshot()
		if len(screenshot) > 0 {
			os.WriteFile("/tmp/gowright-before-save.png", screenshot, 0o644)
			t.Log("Step: Screenshot saved to /tmp/gowright-before-save.png")
		}

		// --- Save & Exit ---
		t.Log("Step: Save & Exit")

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

		page.Evaluate(`(function() { var el = document.querySelector('.intercom-lightweight-app, [class*="freshchat"], [class*="help-widget"], iframe[title*="chat"]'); if (el) el.style.display = 'none'; })()`)
		page.GetByTestId("work-order-save-exit-button").Click()

		screenshot2 := page.Screenshot()
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
		pw.Expect(pw.Page).WithTimeout(15 * time.Second).ToHaveURL("/work-order")
		waitForLoading()

		t.Log("Step: Search for work order")
		searchInput := page.GetByTestId("work-order-search-input")
		searchInput.Click()
		searchInput.Fill(workOrderNumber)
		searchInput.Press("Enter")
		waitForLoading()

		row := page.GetByTestId("work-order-row").Filter(gowright.FilterOptions{
			HasText: workOrderNumber,
		}).First()
		pw.Expect(row).WithTimeout(10 * time.Second).ToBeVisible()
		t.Log("Step: Work order verified in table")
	})
}
