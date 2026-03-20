<script lang="ts">
  interface Props {
    chatInput: string;
    disabled: boolean;
    chatProviderProfiles: any[];
    chatProviderProfileID: string;
    chatThinkingLevel: string;
    onChatInputChange: (value: string) => void;
    onProviderProfileChange: (value: string) => void;
    onThinkingLevelChange: (value: string) => void;
    onSend: () => void;
  }

  let {
    chatInput,
    disabled,
    chatProviderProfiles,
    chatProviderProfileID,
    chatThinkingLevel,
    onChatInputChange,
    onProviderProfileChange,
    onThinkingLevelChange,
    onSend
  }: Props = $props();

  function handleKeydown(event: KeyboardEvent) {
    if (event.key === 'Enter' && !event.shiftKey) {
      event.preventDefault();
      if (!disabled && chatInput.trim() !== '') {
        onSend();
      }
    }
  }
</script>

<div class="composer-shell">
  <textarea
    placeholder="Ask anything"
    value={chatInput}
    oninput={(event) => onChatInputChange((event.currentTarget as HTMLTextAreaElement).value)}
    onkeydown={handleKeydown}
    disabled={disabled}
    class="composer-input"
    rows={2}
  ></textarea>

  <div class="composer-controls">
    <div class="composer-left">
      <select
        class="composer-select"
        value={chatProviderProfileID}
        oninput={(event) => onProviderProfileChange((event.currentTarget as HTMLSelectElement).value)}
        disabled={disabled}
        aria-label="Provider profile"
      >
        {#if chatProviderProfiles.length === 0}
          <option value="">Provider: service default</option>
        {:else}
          {#each chatProviderProfiles as profile}
            <option value={profile.id}>{profile.name || profile.id}</option>
          {/each}
        {/if}
      </select>

      <select
        class="composer-select"
        value={chatThinkingLevel}
        oninput={(event) => onThinkingLevelChange((event.currentTarget as HTMLSelectElement).value)}
        disabled={disabled}
        aria-label="Thinking level"
      >
        <option value="quick">Thinking: quick</option>
        <option value="balanced">Thinking: balanced</option>
        <option value="deep">Thinking: deep</option>
      </select>
    </div>

    <button class="composer-send" type="button" onclick={onSend} disabled={disabled || chatInput.trim() === ''} aria-label="Send">
      ↑
    </button>
  </div>
</div>

<style>
  .composer-shell {
    border: 1px solid #1f2937;
    background: #05070b;
    padding: 10px;
  }

  .composer-input {
    width: 100%;
    border: none;
    outline: none;
    resize: none;
    background: transparent;
    color: #d8dfea;
    font-size: 0.95rem;
    line-height: 1.5;
    min-height: 48px;
    padding: 2px 4px;
  }

  .composer-input::placeholder {
    color: #7a8494;
    font-weight: 500;
  }

  .composer-controls {
    margin-top: 8px;
    display: flex;
    align-items: center;
    justify-content: space-between;
    gap: 8px;
  }

  .composer-left {
    display: inline-flex;
    align-items: center;
    gap: 8px;
    min-width: 0;
    flex: 1;
  }

  .composer-select {
    height: 30px;
    min-width: 0;
    max-width: 208px;
    border: 1px solid #2f3a4d;
    border-radius: 0;
    color: #d7deea;
    background: #0a0f17;
    font-size: 0.72rem;
    font-weight: 600;
    padding: 0 10px;
  }

  .composer-select:disabled {
    opacity: 0.45;
  }

  .composer-send {
    width: 30px;
    height: 30px;
    border: none;
    border-radius: 0;
    background: #86bc25;
    color: #0a0a0a;
    font-size: 0.9rem;
    font-weight: 900;
    display: inline-flex;
    align-items: center;
    justify-content: center;
  }

  .composer-send:disabled {
    opacity: 0.35;
  }
</style>
