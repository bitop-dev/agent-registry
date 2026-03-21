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

  let { kind }: { kind: "plugin" | "profile" } = $props();

  let results = $state<SearchResult[]>([]);
  let loading = $state(true);
  let query = $state("");
  let sortBy = $state("name");

  async function load() {
    loading = true;
    try {
      const resp = await search(query, kind, "", sortBy);
      results = resp.results;
    } catch (e) {
      console.error(e);
    } finally {
      loading = false;
    }
  }

  onMount(load);
</script>

<div class="max-w-5xl mx-auto space-y-6">
  <div class="flex items-center justify-between">
    <h1 class="text-3xl font-bold">
      {kind === "plugin" ? "Plugins" : "Profiles"}
    </h1>
    <div class="flex gap-1">
      {#each [["name", "A-Z"], ["downloads", "Popular"]] as [val, label]}
        <Button
          variant={sortBy === val ? "default" : "outline"}
          size="sm"
          onclick={() => {
            sortBy = val;
            load();
          }}>{label}</Button
        >
      {/each}
    </div>
  </div>

  <div class="relative">
    <SearchIcon
      class="absolute left-3 top-1/2 -translate-y-1/2 h-4 w-4 text-muted-foreground"
    />
    <Input
      placeholder="Filter {kind}s..."
      class="pl-10"
      bind:value={query}
      oninput={() => load()}
    />
  </div>

  <p class="text-sm text-muted-foreground">{results.length} {kind}s</p>

  <div class="grid gap-4 md:grid-cols-2 lg:grid-cols-3">
    {#each results as item (item.name)}
      <button
        class="text-left"
        onclick={() =>
          navigate(
            kind === "plugin"
              ? `/plugins/${item.name}`
              : `/profiles/${item.name}`
          )}
      >
        <Card.Root
          class="h-full hover:border-primary/50 transition-colors cursor-pointer"
        >
          <Card.Header class="pb-2">
            <div class="flex items-center justify-between">
              <div class="flex items-center gap-2">
                {#if kind === "plugin"}
                  <PuzzleIcon class="h-4 w-4 text-muted-foreground" />
                {:else}
                  <BotIcon class="h-4 w-4 text-muted-foreground" />
                {/if}
                <Card.Title class="text-base">{item.name}</Card.Title>
              </div>
              <div class="flex items-center gap-2">
                <span class="flex items-center gap-1 text-xs text-muted-foreground">
                  <DownloadIcon class="h-3 w-3" />
                  {item.downloads}
                </span>
                <Badge variant="outline" class="text-[10px] font-mono"
                  >v{item.version}</Badge
                >
              </div>
            </div>
          </Card.Header>
          <Card.Content class="space-y-2">
            <p class="text-sm text-muted-foreground line-clamp-2">
              {item.description}
            </p>
            <div class="flex flex-wrap gap-1">
              {#if item.runtime}
                <Badge variant="secondary" class="text-[10px]"
                  >{item.runtime}</Badge
                >
              {/if}
              {#if item.category}
                <Badge variant="outline" class="text-[10px]"
                  >{item.category}</Badge
                >
              {/if}
              {#each (item.tools || []).slice(0, 4) as t}
                <Badge variant="outline" class="font-mono text-[10px]"
                  >{t}</Badge
                >
              {/each}
              {#each (item.capabilities || []).slice(0, 3) as c}
                <Badge variant="secondary" class="text-[10px]">{c}</Badge>
              {/each}
            </div>
          </Card.Content>
        </Card.Root>
      </button>
    {/each}
  </div>
</div>
