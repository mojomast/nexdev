import type { ExtensionContext } from "@earendil-works/pi-coding-agent";

import { createNexdevClient, NexdevClientError, redactControlSecrets, redactUnknown } from "./client.js";

type NotifyLevel = "info" | "warning" | "error";

export async function steerNexdev(ctx: ExtensionContext): Promise<void> {
  let client: ReturnType<typeof createNexdevClient>;
  try {
    client = createNexdevClient();
  } catch {
    notify(ctx, "Nexdev control URL is not configured.", "error");
    return;
  }

  const input = await ctx.ui.editor("Steer Nexdev", "");
  if (typeof input !== "string") {
    return;
  }

  const message = input.trim();
  if (message === "") {
    notify(ctx, "Steering message cannot be empty.", "warning");
    return;
  }

  try {
    await client.steer({ message });
    notify(ctx, "Steering message sent to Nexdev", "info");
  } catch (error) {
    notify(ctx, `Failed to send steering message: ${safeErrorMessage(error)}`, "error");
  }
}

function notify(ctx: ExtensionContext, message: string, level: NotifyLevel): void {
  ctx.ui.notify(redactControlSecrets(message), level);
}

function safeErrorMessage(error: unknown): string {
  if (error instanceof NexdevClientError) {
    return error.message;
  }

  const redacted = redactUnknown(error);
  if (redacted instanceof Error) {
    return redactControlSecrets(redacted.message);
  }
  return redactControlSecrets(String(redacted));
}
