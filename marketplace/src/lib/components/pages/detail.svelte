<script lang="ts">
  import { onMount } from "svelte";
  import {
    getPluginDetail,
    getProfileDetail,
    type PackageDetail,
  } from "$lib/api";
  import { navigate } from "$lib/router";
  import * as Card from "$lib/components/ui/card/index.js";
  import { Badge } from "$lib/components/ui/badge/index.js";
  import { Button } from "$lib/components/ui/button/index.js";
  import { Separator } from "$lib/components/ui/separator/index.js";
  import ArrowLeftIcon from "@lucide/svelte/icons/arrow-left";
  import DownloadIcon from "@lucide/svelte/icons/download";
  import CopyIcon from "@lucide/svelte/icons/copy";
  import PuzzleIcon from "@lucide/svelte/icons/puzzle";
  import BotIcon from "@lucide/svelte/icons/bot";
  import WrenchIcon from "@lucide/svelte/icons/wrench";

  let { kind, name }: { kind: "plugin" | "profile"; name: string } = $props();

  let detail = $state<PackageDetail | null>(null);
  let loading = $state(true);
  let error = $state("");

  async function load() {
    loading = true;
    error = "";
    try {
      detail =
        kind === "plugin"
          ? await getPluginDetail(name)
          : await getProfileDetail(name);
    } catch (e: any) {
      error = e.message;
    } finally {
      loading = false;
    }
  }

  onMount(load);
  $effect(() => { load(); });

  let installCmd = $derived(
    kind === "plugin"
      ? `agent plugins install ${name}`
      : `agent profiles install ${name}`
  );

  function copyInstall() {
    navigator.clipboard.writeText(installCmd);
  }
</script>

<div class="max-w-4xl mx-auto space-y-6">
  <Button
    variant="ghost"
    size="sm"
    onclick={() => navigate(kind === "plugin" ? "/plugins" : "/profiles")}
  >
    <ArrowLeftIcon class="h-4 w-4 mr-1" />
    Back to {kind === "plugin" ? "Plugins" : "Profiles"}
  </Button>

  {#if loading}
    <Card.Root>
      <Card.Content class="p-8 text-center text-muted-foreground">
        Loading...
      </Card.Content>
    </Card.Root>
  {:else if error}
    <Card.Root>
      <Card.Content class="p-8 text-center text-destructive">
        {error}
      </Card.Content>
    </Card.Root>
  {:else if detail}
    <!-- Header -->
    <Card.Root>
      <Card.Content class="p-6">
        <div class="flex items-start justify-between">
          <div class="flex items-center gap-3">
            {#if kind === "plugin"}
              <PuzzleIcon class="h-8 w-8 text-primary" />
            {:else}
              <BotIcon class="h-8 w-8 text-primary" />
            {/if}
            <div>
              <h1 class="text-3xl font-bold">{detail.name}</h1>
              <p class="text-muted-foreground mt-1">{detail.description}</p>
            </div>
          </div>
          <div class="text-right space-y-1">
            <Badge variant="outline" class="font-mono">v{detail.version}</Badge>
            <div
              class="flex items-center gap-1 text-sm text-muted-foreground justify-end"
            >
              <DownloadIcon class="h-3 w-3" />
              {detail.downloads} downloads
            </div>
          </div>
        </div>

        <Separator class="my-4" />

        <!-- Install command -->
        <div
          class="flex items-center gap-2 bg-muted rounded-md p-3 font-mono text-sm"
        >
          <span class="text-muted-foreground">$</span>
          <span class="flex-1">{installCmd}</span>
          <Button variant="ghost" size="sm" class="h-7" onclick={copyInstall}>
            <CopyIcon class="h-3 w-3" />
          </Button>
        </div>
      </Card.Content>
    </Card.Root>

    <!-- Metadata grid -->
    <div class="grid gap-4 md:grid-cols-2">
      {#if detail.tools?.length}
        <Card.Root>
          <Card.Header class="pb-2">
            <Card.Title class="text-sm flex items-center gap-1">
              <WrenchIcon class="h-4 w-4" />
              Tools
            </Card.Title>
          </Card.Header>
          <Card.Content>
            <div class="flex flex-wrap gap-1.5">
              {#each detail.tools as t}
                <Badge variant="secondary" class="font-mono">{t}</Badge>
              {/each}
            </div>
          </Card.Content>
        </Card.Root>
      {/if}

      {#if detail.capabilities?.length}
        <Card.Root>
          <Card.Header class="pb-2">
            <Card.Title class="text-sm">Capabilities</Card.Title>
          </Card.Header>
          <Card.Content>
            <div class="flex flex-wrap gap-1.5">
              {#each detail.capabilities as c}
                <Badge variant="secondary">{c}</Badge>
              {/each}
            </div>
          </Card.Content>
        </Card.Root>
      {/if}

      <Card.Root>
        <Card.Header class="pb-2">
          <Card.Title class="text-sm">Details</Card.Title>
        </Card.Header>
        <Card.Content class="space-y-2 text-sm">
          {#if detail.category}
            <div>
              <span class="text-muted-foreground">Category:</span>
              <Badge variant="outline" class="ml-1">{detail.category}</Badge>
            </div>
          {/if}
          {#if detail.runtime}
            <div>
              <span class="text-muted-foreground">Runtime:</span>
              <Badge variant="outline" class="ml-1">{detail.runtime}</Badge>
            </div>
          {/if}
          {#if detail.model}
            <div>
              <span class="text-muted-foreground">Model:</span>
              <span class="font-mono ml-1">{detail.model}</span>
            </div>
          {/if}
          {#if detail.provider}
            <div>
              <span class="text-muted-foreground">Provider:</span>
              <span class="ml-1">{detail.provider}</span>
            </div>
          {/if}
          {#if detail.extends}
            <div>
              <span class="text-muted-foreground">Extends:</span>
              <button
                class="ml-1 text-primary underline"
                onclick={() => navigate(`/profiles/${detail?.extends}`)}
                >{detail.extends}</button
              >
            </div>
          {/if}
          {#if detail.mode}
            <div>
              <span class="text-muted-foreground">Mode:</span>
              <span class="ml-1">{detail.mode}</span>
            </div>
          {/if}
          {#if detail.accepts}
            <div>
              <span class="text-muted-foreground">Accepts:</span>
              <span class="ml-1">{detail.accepts}</span>
            </div>
          {/if}
          {#if detail.returns}
            <div>
              <span class="text-muted-foreground">Returns:</span>
              <span class="ml-1">{detail.returns}</span>
            </div>
          {/if}
          {#if detail.dependencies?.length}
            <div>
              <span class="text-muted-foreground">Dependencies:</span>
              {#each detail.dependencies as dep}
                <Badge variant="outline" class="ml-1">{dep}</Badge>
              {/each}
            </div>
          {/if}
        </Card.Content>
      </Card.Root>
    </div>

    <!-- README -->
    {#if detail.readme}
      <Card.Root>
        <Card.Header>
          <Card.Title>README</Card.Title>
        </Card.Header>
        <Card.Content>
          <div class="prose prose-sm dark:prose-invert max-w-none whitespace-pre-wrap">
            {detail.readme}
          </div>
        </Card.Content>
      </Card.Root>
    {/if}
  {/if}
</div>
