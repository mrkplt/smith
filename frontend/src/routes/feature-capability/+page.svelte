<script lang="ts">
  import { onDestroy, onMount } from 'svelte';
  import { goto } from '$app/navigation';
  import { Button, Label } from 'flowbite-svelte';
  import TopBar from '$lib/components/TopBar.svelte';
  import { hasFeatureCapabilityAccess } from '$lib/feature-capability/access';
  import {
    executeFeatureCapabilityTask,
    FeatureCapabilityExecutionError,
    getFeatureCapabilityRunStatus,
    isTerminalRunStatus,
    retryFeatureCapabilityRun
  } from '$lib/feature-capability/execution';
  import {
    type FeatureCapabilityInputs,
    type ValidationErrors,
    validateFeatureCapabilityInputs
  } from '$lib/feature-capability/validation';

  const STORAGE_KEY = 'feature-capability-flow-state';

  type AttemptHistoryItem = {
    kind: 'execute' | 'retry';
    startedAt: string;
    status: string;
    reference: string;
  };

  let currentStep = $state(1);
  let inputs = $state<FeatureCapabilityInputs>({
    taskName: '',
    targetEnvironment: ''
  });
  let errors = $state<ValidationErrors>({});
  let executionState = $state<'idle' | 'submitting' | 'submitted' | 'failed'>('idle');
  let executionError = $state('');
  let executionMessage = $state('');
  let confirmationId = $state('');
  let hasExecuted = $state(false);
  let runId = $state('');
  let runStatus = $state('');
  let retryAvailable = $state(false);
  let statusRefreshError = $state('');
  let attemptHistory = $state<AttemptHistoryItem[]>([]);

  let statusPollTimer: ReturnType<typeof setInterval> | null = null;

  function clearPolling() {
    if (!statusPollTimer) {
      return;
    }
    clearInterval(statusPollTimer);
    statusPollTimer = null;
  }

  function updateLatestAttemptStatus(status: string) {
    if (attemptHistory.length === 0) {
      return;
    }

    const nextHistory = [...attemptHistory];
    const latest = nextHistory[nextHistory.length - 1];
    if (latest.status === status) {
      return;
    }

    nextHistory[nextHistory.length - 1] = {
      ...latest,
      status
    };
    attemptHistory = nextHistory;
  }

  async function refreshRunStatus() {
    if (!runId) {
      return;
    }

    try {
      const latest = await getFeatureCapabilityRunStatus(runId);
      runStatus = latest.status;
      retryAvailable = latest.retryAvailable;
      updateLatestAttemptStatus(latest.status);
      statusRefreshError = '';

      if (isTerminalRunStatus(latest.status)) {
        clearPolling();
        if (latest.status === 'SUCCEEDED') {
          executionMessage = 'Execution completed successfully.';
        } else if (latest.status === 'FAILED_RECOVERABLE') {
          executionMessage = 'Execution failed with a recoverable issue. You can retry this run.';
        } else {
          executionMessage = 'Execution finished with a terminal failure.';
        }
      }
    } catch {
      statusRefreshError = 'Unable to refresh execution status. The latest known state is still shown.';
    }
  }

  function startPollingIfExecuting() {
    clearPolling();
    if (!runId || runStatus !== 'EXECUTING') {
      return;
    }

    statusPollTimer = setInterval(() => {
      void refreshRunStatus();
    }, 5000);
  }

  onMount(() => {
    if (!hasFeatureCapabilityAccess()) {
      void goto('/feature-capability/access-denied');
      return;
    }

    const raw = window.localStorage.getItem(STORAGE_KEY);
    if (!raw) {
      return;
    }

    try {
      const parsed = JSON.parse(raw) as {
        currentStep?: number;
        inputs?: FeatureCapabilityInputs;
        executionState?: 'idle' | 'submitting' | 'submitted' | 'failed';
        executionError?: string;
        executionMessage?: string;
        confirmationId?: string;
        hasExecuted?: boolean;
        runId?: string;
        runStatus?: string;
        retryAvailable?: boolean;
        statusRefreshError?: string;
        attemptHistory?: AttemptHistoryItem[];
      };

      if (parsed.currentStep === 1 || parsed.currentStep === 2 || parsed.currentStep === 3) {
        currentStep = parsed.currentStep;
      }
      if (parsed.inputs) {
        inputs = {
          taskName: parsed.inputs.taskName || '',
          targetEnvironment: parsed.inputs.targetEnvironment || ''
        };
      }
      if (parsed.executionState) {
        executionState = parsed.executionState;
      }
      executionError = parsed.executionError || '';
      executionMessage = parsed.executionMessage || '';
      confirmationId = parsed.confirmationId || '';
      hasExecuted = parsed.hasExecuted || false;
      runId = parsed.runId || '';
      runStatus = parsed.runStatus || '';
      retryAvailable = parsed.retryAvailable === true;
      statusRefreshError = parsed.statusRefreshError || '';
      attemptHistory = Array.isArray(parsed.attemptHistory) ? parsed.attemptHistory : [];

      if (runStatus === 'EXECUTING' && runId) {
        startPollingIfExecuting();
        void refreshRunStatus();
      }
    } catch {
      window.localStorage.removeItem(STORAGE_KEY);
    }
  });

  onDestroy(() => {
    clearPolling();
  });

  $effect(() => {
    if (typeof window === 'undefined') {
      return;
    }

    window.localStorage.setItem(
      STORAGE_KEY,
      JSON.stringify({
        currentStep,
        inputs,
        executionState,
        executionError,
        executionMessage,
        confirmationId,
        hasExecuted,
        runId,
        runStatus,
        retryAvailable,
        statusRefreshError,
        attemptHistory
      })
    );
  });

  function continueToNextStep() {
    const validation = validateFeatureCapabilityInputs(inputs);
    errors = validation;
    if (Object.keys(validation).length > 0) {
      return;
    }
    currentStep = 2;
  }

  async function submitExecution() {
    if (executionState === 'submitting' || hasExecuted) {
      return;
    }

    executionState = 'submitting';
    executionError = '';

    try {
      const result = await executeFeatureCapabilityTask(inputs);
      hasExecuted = true;
      executionMessage = result.message;
      confirmationId = result.confirmationId;
      runId = result.runId || '';
      runStatus = result.status;
      retryAvailable = false;
      executionState = 'submitted';
      currentStep = 3;

      attemptHistory = [
        ...attemptHistory,
        {
          kind: 'execute',
          startedAt: new Date().toISOString(),
          status: result.status,
          reference: result.confirmationId
        }
      ];

      startPollingIfExecuting();
      if (runStatus === 'EXECUTING' && runId) {
        void refreshRunStatus();
      }
    } catch (error) {
      executionState = 'failed';
      if (error instanceof FeatureCapabilityExecutionError) {
        executionError = error.message;
        return;
      }
      executionError = 'Execution could not be started. Try again after verifying your network connection.';
    }
  }

  async function submitRetry() {
    if (executionState === 'submitting' || !runId || runStatus !== 'FAILED_RECOVERABLE' || !retryAvailable) {
      return;
    }

    executionState = 'submitting';
    executionError = '';

    try {
      const result = await retryFeatureCapabilityRun(runId);
      runStatus = result.status;
      executionMessage = result.message;
      retryAvailable = false;
      executionState = 'submitted';

      attemptHistory = [
        ...attemptHistory,
        {
          kind: 'retry',
          startedAt: new Date().toISOString(),
          status: result.status,
          reference: result.attemptId || result.idempotencyKey
        }
      ];

      startPollingIfExecuting();
      if (runStatus === 'EXECUTING') {
        void refreshRunStatus();
      }
    } catch (error) {
      executionState = 'failed';
      if (error instanceof FeatureCapabilityExecutionError) {
        executionError = error.message;
        return;
      }
      executionError = 'Retry request failed. Check your connection and try again.';
    }
  }
</script>

<TopBar title="Feature Capability" />

<section class="px-4 pb-8">
  <div class="border border-gray-800 bg-black/60 p-6 md:p-8 max-w-4xl">
    <div class="text-[10px] uppercase tracking-[0.2em] font-bold text-[#86BC25]">Step {currentStep} of 3</div>
    {#if currentStep === 1}
      <h2 class="mt-3 text-xl font-bold text-white uppercase tracking-tight">Required Inputs</h2>
      <p class="mt-3 text-sm text-gray-400 max-w-2xl">
        Fill in all required fields before continuing.
      </p>

      <div class="mt-6 space-y-5">
        <div>
          <Label class="mb-2 text-gray-300 uppercase font-bold text-xs tracking-widest">
            Task Name <span class="text-red-400">*</span>
          </Label>
          <input
            type="text"
            class="w-full bg-slate-900 border text-white text-sm rounded-none px-3 py-2 {errors.taskName ? 'border-red-500' : 'border-gray-800'}"
            placeholder="Example: Prepare deployment report"
            bind:value={inputs.taskName}
          />
          {#if errors.taskName}
            <p class="mt-1 text-xs text-red-400">{errors.taskName}</p>
          {/if}
        </div>

        <div>
          <Label class="mb-2 text-gray-300 uppercase font-bold text-xs tracking-widest">
            Target Environment <span class="text-red-400">*</span>
          </Label>
          <input
            type="text"
            class="w-full bg-slate-900 border text-white text-sm rounded-none px-3 py-2 {errors.targetEnvironment ? 'border-red-500' : 'border-gray-800'}"
            placeholder="Example: staging"
            bind:value={inputs.targetEnvironment}
          />
          {#if errors.targetEnvironment}
            <p class="mt-1 text-xs text-red-400">{errors.targetEnvironment}</p>
          {/if}
        </div>
      </div>

      <div class="mt-6 flex gap-3">
        <Button color="alternative" class="rounded-none bg-[#86BC25] text-black font-bold" onclick={continueToNextStep}>
          Continue
        </Button>
        <Button color="alternative" class="rounded-none border-gray-700" href="/pods">
          Cancel
        </Button>
      </div>
    {:else if currentStep === 2}
      <h2 class="mt-3 text-xl font-bold text-white uppercase tracking-tight">Review and Execute</h2>
      <p class="mt-3 text-sm text-gray-400 max-w-2xl">
        Required inputs validated successfully. Submit to execute the task.
      </p>
      <div class="mt-6 border border-gray-800 p-4 bg-black space-y-2 text-sm">
        <div><span class="text-gray-500 uppercase text-[10px] font-bold tracking-widest">Task Name</span></div>
        <div class="text-white">{inputs.taskName}</div>
        <div class="pt-2"><span class="text-gray-500 uppercase text-[10px] font-bold tracking-widest">Target Environment</span></div>
        <div class="text-white">{inputs.targetEnvironment}</div>
      </div>
      {#if executionError}
        <div class="mt-4 border border-red-900 bg-red-950/30 px-4 py-3 text-sm text-red-200">
          {executionError}
        </div>
      {/if}
      <div class="mt-6">
        <Button
          color="alternative"
          class="rounded-none bg-[#86BC25] text-black font-bold"
          onclick={submitExecution}
          disabled={executionState === 'submitting' || hasExecuted}
        >
          {executionState === 'submitting' ? 'Submitting...' : (hasExecuted ? 'Submitted' : 'Submit')}
        </Button>
        <Button
          color="alternative"
          class="rounded-none border-gray-700 ml-3"
          onclick={() => currentStep = 1}
          disabled={executionState === 'submitting' || hasExecuted}
        >
          Back
        </Button>
      </div>
    {:else}
      <h2 class="mt-3 text-xl font-bold text-white uppercase tracking-tight">
        {runStatus === 'EXECUTING' ? 'Execution In Progress' : 'Execution Status'}
      </h2>
      <p class="mt-3 text-sm text-gray-400 max-w-2xl">
        {runStatus === 'EXECUTING'
          ? 'The task is still running. This status will continue updating and remains visible after refresh.'
          : (executionMessage || 'Execution status is available below.')}
      </p>

      <div class="mt-6 border p-4 text-sm {runStatus === 'SUCCEEDED' ? 'border-[#86BC25]/60 bg-[#86BC25]/10' : (runStatus === 'FAILED_RECOVERABLE' || runStatus === 'FAILED_TERMINAL' ? 'border-red-900 bg-red-950/30' : 'border-gray-700 bg-black')}" >
        <div class="text-[10px] uppercase tracking-[0.2em] font-bold {runStatus === 'SUCCEEDED' ? 'text-[#86BC25]' : (runStatus === 'FAILED_RECOVERABLE' || runStatus === 'FAILED_TERMINAL' ? 'text-red-300' : 'text-gray-300')}">
          {runStatus || 'EXECUTING'}
        </div>
        {#if confirmationId}
          <div class="mt-2 text-gray-300">Confirmation ID</div>
          <div class="mt-1 text-white font-mono">{confirmationId}</div>
        {/if}
      </div>

      {#if executionError}
        <div class="mt-4 border border-red-900 bg-red-950/30 px-4 py-3 text-sm text-red-200">
          {executionError}
        </div>
      {/if}

      {#if runStatus === 'FAILED_RECOVERABLE' && retryAvailable}
        <div class="mt-4">
          <Button
            color="alternative"
            class="rounded-none bg-[#86BC25] text-black font-bold"
            onclick={submitRetry}
            disabled={executionState === 'submitting'}
          >
            {executionState === 'submitting' ? 'Retrying...' : 'Retry Execution'}
          </Button>
        </div>
      {/if}

      {#if statusRefreshError}
        <div class="mt-4 border border-yellow-900 bg-yellow-950/30 px-4 py-3 text-sm text-yellow-200">
          {statusRefreshError}
        </div>
      {/if}

      {#if attemptHistory.length > 0}
        <div class="mt-6 border border-gray-800 p-4 bg-black/70">
          <div class="text-[10px] uppercase tracking-[0.2em] font-bold text-gray-300">Attempt History</div>
          <div class="mt-3 space-y-2 text-sm">
            {#each attemptHistory as attempt}
              <div class="border border-gray-800 p-3">
                <div class="text-white uppercase text-xs font-bold tracking-widest">{attempt.kind}</div>
                <div class="mt-1 text-gray-300">Status: {attempt.status}</div>
                <div class="text-gray-400">Started: {attempt.startedAt}</div>
                <div class="text-gray-400">Reference: <span class="font-mono">{attempt.reference}</span></div>
              </div>
            {/each}
          </div>
        </div>
      {/if}

      <div class="mt-6">
        <Button color="alternative" class="rounded-none border-gray-700" href="/pods">
          Return to Pods
        </Button>
      </div>
    {/if}
  </div>
</section>
