<script lang="ts">
  import { onMount } from "svelte";
  import { search, type SearchResult } from "$lib/api";
  import { navigate } from "$lib/router";
  import * as Card from "$lib/components/ui/card/index.js";
  import { Badge } from "$lib/components/ui/badge/index.js";
  import { Button } from "$lib/components/ui/button/index.js";
  import { Input } from "$lib/components/ui/input/index.js";
  import SearchIcon from "@lucide/svelte/icons/search";
  import PuzzleIcon from "@lucide/svelte/icons/puzzle";
  import BotIcon from "@lucide/svelte/icons/bot";
  import DownloadIcon from "@lucide/svelte/icons/download";
  import ArrowRightIcon from "@lucide/svelte/icons/arrow-right";

  let popular = $state<SearchResult[]>([]);
  let recent = $state<SearchResult[]>([]);
  let query = $state("");
  let loading = $state(true);
  let stats = $state({ plugins: 0, profiles: 0 });

  onMount(async () => {
    try {
      const [popResp, allResp] = await Promise.all([
        search("", "", "", "downloads"),
        search(""),
      ]);
      popular = popResp.results.slice(0, 6);
      recent = allResp.results.slice(0, 6);
      stats.plugins = allResp.results.filter((r) => r.type === "plugin").length;
      stats.profiles = allResp.results.filter((r) => r.type === "profile").length;
    } catch (e) {
      console.error(e);
    } finally {
      loading = false;
    }
  });

  function doSearch() {
    if (query.trim()) {
      navigate(`/search?q=${encodeURIComponent(query.trim())}`);
    }
  }
</script>

<div class="max-w-5xl mx-auto space-y-12">
  <!-- Hero -->
  <div class="text-center space-y-6 py-12">
    <h1 class="text-5xl font-bold tracking-tight">
      Agent Marketplace
    </h1>
    <p class="text-xl text-muted-foreground max-w-2xl mx-auto">
      Discover plugins and profiles for the Agent platform.
      {stats.plugins} plugins and {stats.profiles} profiles available.
    </p>

    <!-- Search -->
    <div class="flex max-w-lg mx-auto gap-2">
      <div class="relative flex-1">
        <SearchIcon
          class="absolute left-3 top-1/2 -translate-y-1/2 h-4 w-4 text-muted-foreground"
        />
        <Input
          placeholder="Search plugins and profiles..."
          class="pl-10"
          bind:value={query}
          onkeydown={(e: KeyboardEvent) => e.key === "Enter" && doSearch()}
        />
      </div>
      <Button onclick={doSearch}>Search</Button>
    </div>

    <!-- Quick links -->
    <div class="flex justify-center gap-3">
      <Button variant="outline" size="sm" onclick={() => navigate("/plugins")}>
        <PuzzleIcon class="h-4 w-4 mr-1" />
        Browse Plugins
      </Button>
      <Button variant="outline" size="sm" onclick={() => navigate("/profiles")}>
        <BotIcon class="h-4 w-4 mr-1" />
        Browse Profiles
      </Button>
    </div>
  </div>

  <!-- Popular -->
  <section>
    <div class="flex items-center justify-between mb-4">
      <h2 class="text-2xl font-bold">Popular</h2>
      <Button
        variant="ghost"
        size="sm"
        onclick={() => navigate("/search?sort=downloads")}
      >
        View all <ArrowRightIcon class="h-4 w-4 ml-1" />
      </Button>
    </div>
    <div class="grid gap-4 md:grid-cols-2 lg:grid-cols-3">
      {#each popular as item (item.name)}
        <button
          class="text-left"
          onclick={() =>
            navigate(
              item.type === "plugin"
                ? `/plugins/${item.name}`
                : `/profiles/${item.name}`
            )}
        >
          <Card.Root class="h-full hover:border-primary/50 transition-colors cursor-pointer">
            <Card.Header class="pb-2">
              <div class="flex items-center justify-between">
                <div class="flex items-center gap-2">
                  {#if item.type === "plugin"}
                    <PuzzleIcon class="h-4 w-4 text-muted-foreground" />
                  {:else}
                    <BotIcon class="h-4 w-4 text-muted-foreground" />
                  {/if}
                  <Card.Title class="text-base">{item.name}</Card.Title>
                </div>
                <div class="flex items-center gap-1 text-xs text-muted-foreground">
                  <DownloadIcon class="h-3 w-3" />
                  {item.downloads}
                </div>
              </div>
            </Card.Header>
            <Card.Content class="space-y-2">
              <p class="text-sm text-muted-foreground line-clamp-2">
                {item.description}
              </p>
              <div class="flex flex-wrap gap-1">
                <Badge variant="outline" class="text-[10px]">{item.type}</Badge>
                {#if item.runtime}
                  <Badge variant="secondary" class="text-[10px]"
                    >{item.runtime}</Badge
                  >
                {/if}
                {#each (item.tools || []).slice(0, 3) as t}
                  <Badge variant="outline" class="font-mono text-[10px]"
                    >{t}</Badge
                  >
                {/each}
              </div>
            </Card.Content>
          </Card.Root>
        </button>
      {/each}
    </div>
  </section>
</div>
