import { describe, it, expect, vi } from "vitest";
import { screen, act } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import ApplicationNew from "../../../app/routes/eam/application-new";
import { renderWithRouter } from "../../render-helper";

function renderNewForm() {
  return renderWithRouter(
    <ApplicationNew
      loaderData={{ isNew: true }}
      params={{}}
      matches={[]}
      actionData={undefined}
    />
  );
}

describe("ApplicationNew", () => {
  it("renders heading", () => {
    renderNewForm();
    expect(screen.getByText("New Application")).toBeInTheDocument();
  });

  it("renders all form field labels", () => {
    const { container } = renderNewForm();
    const text = container.textContent ?? "";
    expect(text).toContain("Name (unique identifier)");
    expect(text).toContain("Display Name");
    expect(text).toContain("Description");
    expect(text).toContain("Status");
    expect(text).toContain("Business Criticality");
    expect(text).toContain("Technical Risk");
    expect(text).toContain("Lifecycle Phase");
    expect(text).toContain("TIME Category");
    expect(text).toContain("End of Life Date");
    expect(text).toContain("Tags (comma-separated)");
  });

  it("has Create and Cancel buttons", () => {
    const { container } = renderNewForm();
    const text = container.textContent ?? "";
    expect(text).toContain("Create");
    expect(text).toContain("Cancel");
  });

  it("shows Saving... during submit", async () => {
    const user = userEvent.setup();

    vi.stubGlobal(
      "fetch",
      vi.fn().mockImplementation(
        () =>
          new Promise((resolve) =>
            setTimeout(
              () =>
                resolve({
                  ok: true,
                  json: () => Promise.resolve({ id: "test-id" }),
                }),
              200
            )
          )
      )
    );

    const { container } = renderNewForm();

    const nameInput = container.querySelector('input[name="name"]') as HTMLInputElement;
    await user.type(nameInput, "test-app");

    const submitBtn = container.querySelector('button[type="submit"]') as HTMLButtonElement;
    await user.click(submitBtn);

    expect(container.textContent).toContain("Saving...");

    vi.restoreAllMocks();
  });

  it("shows alert on error", async () => {
    const user = userEvent.setup();

    vi.stubGlobal(
      "fetch",
      vi.fn().mockImplementation(() =>
        Promise.resolve({
          ok: false,
          text: () => Promise.resolve("Server error"),
        })
      )
    );

    const alertFn = vi.fn();
    vi.stubGlobal("alert", alertFn);

    const { container } = renderNewForm();
    const nameInput = container.querySelector('input[name="name"]') as HTMLInputElement;
    await user.type(nameInput, "test-app");

    const submitBtn = container.querySelector('button[type="submit"]') as HTMLButtonElement;
    await user.click(submitBtn);

    await vi.waitFor(() => {
      expect(alertFn).toHaveBeenCalledWith(
        expect.stringContaining("Failed to save")
      );
    });

    vi.restoreAllMocks();
  });
});
