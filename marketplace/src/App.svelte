<script lang="ts">
  import "./app.css";
  import { ModeWatcher, toggleMode } from "mode-watcher";
  import { Button } from "$lib/components/ui/button/index.js";
  import { currentPath, navigate } from "$lib/router";
  import SunIcon from "@lucide/svelte/icons/sun";
  import MoonIcon from "@lucide/svelte/icons/moon";
  import PuzzleIcon from "@lucide/svelte/icons/puzzle";
  import BotIcon from "@lucide/svelte/icons/bot";
  import HomeIcon from "@lucide/svelte/icons/house";
  import SearchIcon from "@lucide/svelte/icons/search";

  import Home from "$lib/components/pages/home.svelte";
  import Browse from "$lib/components/pages/browse.svelte";
  import Detail from "$lib/components/pages/detail.svelte";
  import SearchResults from "$lib/components/pages/search-results.svelte";

  function getRouteParams(
    path: string
  ): { page: string; kind?: string; name?: string } {
    if (path === "/" || path === "") return { page: "home" };
    if (path === "/plugins") return { page: "browse", kind: "plugin" };
    if (path === "/profiles") return { page: "browse", kind: "profile" };
    if (path.startsWith("/plugins/"))
      return { page: "detail", kind: "plugin", name: path.split("/")[2] };
    if (path.startsWith("/profiles/"))
      return { page: "detail", kind: "profile", name: path.split("/")[2] };
    if (path.startsWith("/search")) return { page: "search" };
    return { page: "404" };
  }

  let route = $derived(getRouteParams($currentPath));
</script>

<ModeWatcher defaultMode="dark" />

<div class="min-h-screen bg-background">
  <!-- Nav bar -->
  <nav
    class="sticky top-0 z-10 border-b bg-background/95 backdrop-blur supports-[backdrop-filter]:bg-background/60"
  >
    <div class="max-w-5xl mx-auto flex items-center justify-between px-6 py-3">
      <div class="flex items-center gap-6">
        <button
          class="flex items-center gap-2 font-bold text-lg"
          onclick={() => navigate("/")}
        >
          <PuzzleIcon class="h-5 w-5 text-primary" />
          Agent Marketplace
        </button>
        <div class="hidden md:flex items-center gap-1">
          <Button
            variant={$currentPath === "/plugins" ? "secondary" : "ghost"}
            size="sm"
            onclick={() => navigate("/plugins")}
          >
            <PuzzleIcon class="h-4 w-4 mr-1" />
            Plugins
          </Button>
          <Button
            variant={$currentPath === "/profiles" ? "secondary" : "ghost"}
            size="sm"
            onclick={() => navigate("/profiles")}
          >
            <BotIcon class="h-4 w-4 mr-1" />
            Profiles
          </Button>
        </div>
      </div>
      <div class="flex items-center gap-2">
        <Button
          variant="ghost"
          size="icon"
          class="h-8 w-8"
          onclick={toggleMode}
        >
          <SunIcon
            class="h-4 w-4 scale-100 rotate-0 transition-all dark:scale-0 dark:-rotate-90"
          />
          <MoonIcon
            class="absolute h-4 w-4 scale-0 rotate-90 transition-all dark:scale-100 dark:rotate-0"
          />
        </Button>
      </div>
    </div>
  </nav>

  <!-- Page content -->
  <main class="px-6 py-8">
    {#if route.page === "home"}
      <Home />
    {:else if route.page === "browse" && route.kind}
      <Browse kind={route.kind} />
    {:else if route.page === "detail" && route.kind && route.name}
      <Detail kind={route.kind} name={route.name} />
    {:else if route.page === "search"}
      <SearchResults />
    {:else}
      <div class="text-center py-12">
        <p class="text-2xl font-bold">404</p>
        <p class="text-muted-foreground">Page not found</p>
      </div>
    {/if}
  </main>

  <!-- Footer -->
  <footer class="border-t py-6 text-center text-sm text-muted-foreground">
    Agent Platform Marketplace
  </footer>
</div>
