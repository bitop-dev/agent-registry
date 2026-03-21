<script lang="ts">
  import { onMount } from "svelte";
  import { search, type SearchResult } from "$lib/api";
  import { navigate, queryParams } from "$lib/router";
  import * as Card from "$lib/components/ui/card/index.js";
  import { Badge } from "$lib/components/ui/badge/index.js";
  import { Button } from "$lib/components/ui/button/index.js";
  import { Input } from "$lib/components/ui/input/index.js";
  import SearchIcon from "@lucide/svelte/icons/search";
  import PuzzleIcon from "@lucide/svelte/icons/puzzle";
  import BotIcon from "@lucide/svelte/icons/bot";
  import DownloadIcon from "@lucide/svelte/icons/download";

  let results = $state<SearchResult[]>([]);
  let loading = $state(true);
  let query = $state($queryParams.get("q") || "");
  let filterType = $state($queryParams.get("type") || "");
  let sortBy = $state($queryParams.get("sort") || "name");

  async function doSearch() {
    loading = true;
    try {
      const resp = await search(query, filterType, "", sortBy);
      results = resp.results;
    } catch (e) {
      console.error(e);
    } finally {
      loading = false;
    }
  }

  onMount(doSearch);

  $effect(() => {
    // Re-search when params change
    query = $queryParams.get("q") || "";
    filterType = $queryParams.get("type") || "";
    sortBy = $queryParams.get("sort") || "name";
    doSearch();
  });
</script>

<div class="max-w-5xl mx-auto space-y-6">
  <h1 class="text-3xl font-bold">Search</h1>

  <div class="flex gap-2">
    <div class="relative flex-1">
      <SearchIcon
        class="absolute left-3 top-1/2 -translate-y-1/2 h-4 w-4 text-muted-foreground"
      />
      <Input
        placeholder="Search..."
        class="pl-10"
        bind:value={query}
        onkeydown={(e: KeyboardEvent) => {
          if (e.key === "Enter")
            navigate(
              `/search?q=${encodeURIComponent(query)}&type=${filterType}&sort=${sortBy}`
            );
        }}
      />
    </div>
    <div class="flex gap-1">
      {#each [["", "All"], ["plugin", "Plugins"], ["profile", "Profiles"]] as [val, label]}
        <Button
          variant={filterType === val ? "default" : "outline"}
          size="sm"
          onclick={() => {
            filterType = val;
            navigate(
              `/search?q=${encodeURIComponent(query)}&type=${val}&sort=${sortBy}`
            );
          }}>{label}</Button
        >
      {/each}
    </div>
    <div class="flex gap-1">
      {#each [["name", "Name"], ["downloads", "Popular"]] as [val, label]}
        <Button
          variant={sortBy === val ? "default" : "outline"}
          size="sm"
          onclick={() => {
            sortBy = val;
            navigate(
              `/search?q=${encodeURIComponent(query)}&type=${filterType}&sort=${val}`
            );
          }}>{label}</Button
        >
      {/each}
    </div>
  </div>

  <p class="text-sm text-muted-foreground">{results.length} results</p>

  <div class="grid gap-4 md:grid-cols-2 lg:grid-cols-3">
    {#each results as item (item.name + item.type)}
      <button
        class="text-left"
        onclick={() =>
          navigate(
            item.type === "plugin"
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
                {#if item.type === "plugin"}
                  <PuzzleIcon class="h-4 w-4 text-muted-foreground" />
                {:else}
                  <BotIcon class="h-4 w-4 text-muted-foreground" />
                {/if}
                <Card.Title class="text-base">{item.name}</Card.Title>
              </div>
              <div
                class="flex items-center gap-1 text-xs text-muted-foreground"
              >
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
