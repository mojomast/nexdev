import type { ExtensionAPI, ExtensionContext } from "@earendil-works/pi-coding-agent";
import { openNexdevMenu } from "./menu.js";

import {
  canUseNexdevWidgets,
  hideNexdevMenuHint,
  startNexdevWidgets,
  stopNexdevWidgets,
} from "./widgets.js";

const EXTENSION_STATUS_KEY = "nexdev.extension";

function canUseTUI(ctx: ExtensionContext): boolean {
  return canUseNexdevWidgets(ctx);
}

function hideMenuHint(ctx: ExtensionContext): void {
  hideNexdevMenuHint(ctx);
}

async function openMenu(ctx: ExtensionContext): Promise<void> {
  await openNexdevMenu(ctx, { onFirstSuccessfulOpen: () => hideMenuHint(ctx) });
}

function showDiagnostic(ctx: ExtensionContext): void {
  if (!canUseTUI(ctx)) {
    return;
  }

  const message = "Nexdev Pi extension loaded. Press Ctrl+N or use /nexdev to open the control menu.";

  ctx.ui.notify(message, "info");
}

export default function nexdevExtension(pi: ExtensionAPI): void {
  pi.registerCommand("nexdev", {
    description: "Open the Nexdev control menu",
    handler: async (_args, ctx) => {
      await openMenu(ctx);
    },
  });

  try {
    pi.registerShortcut("ctrl+n", {
      description: "Open the Nexdev control menu",
      handler: async (ctx) => {
        await openMenu(ctx);
      },
    });
  } catch {
    // Ctrl+N can conflict with host/user bindings; /nexdev remains the required fallback.
  }

  pi.on("session_start", (_event, ctx) => {
    if (!canUseTUI(ctx)) {
      return;
    }

    ctx.ui.setStatus(EXTENSION_STATUS_KEY, "Nexdev: Ctrl+N or /nexdev opens menu");
    startNexdevWidgets(ctx);
    showDiagnostic(ctx);
  });

  pi.on("session_shutdown", (_event, ctx) => {
    if (!canUseTUI(ctx)) {
      return;
    }

    ctx.ui.setStatus(EXTENSION_STATUS_KEY, undefined);
    stopNexdevWidgets(ctx);
  });
}
