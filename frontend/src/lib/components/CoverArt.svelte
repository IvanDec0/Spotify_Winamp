<script lang="ts">
  interface Props {
    cover: string;
    title: string;
    size?: number;
    class?: string;
  }

  let { cover, title, size = 88, class: className = "" }: Props = $props();

  const variants = ["meridian", "tides", "orbit", "afterimage", "bloom", "nightbus"] as const;

  let variant = $derived(
    variants.includes(cover as (typeof variants)[number]) ? cover : "meridian",
  );
</script>

<div
  class={`cover-art cover-art--${variant} ${className}`}
  style={`--art-size: ${size}px`}
  role="img"
  aria-label={`Album art for ${title}`}
>
  <span class="cover-art__wash" aria-hidden="true"></span>

  <svg class="cover-art__glyph" viewBox="0 0 100 100" fill="none" aria-hidden="true">
    {#if variant === "meridian"}
      <circle cx="50" cy="50" r="26" stroke="currentColor" stroke-width="1.1" opacity=".62" />
      <circle cx="50" cy="50" r="16" stroke="currentColor" stroke-width=".7" opacity=".28" />
      <path d="M50 10v80" stroke="currentColor" stroke-width="1.2" opacity=".9" />
      <path d="M14 50h72" stroke="currentColor" stroke-width=".6" opacity=".28" />
      <circle cx="50" cy="50" r="2.7" fill="currentColor" stroke="none" />
    {:else if variant === "tides"}
      <path
        d="M8 57c13-11 25-11 38 0s25 11 46-2"
        stroke="currentColor"
        stroke-width="1.2"
        opacity=".8"
      />
      <path
        d="M8 68c13-11 25-11 38 0s25 11 46-2"
        stroke="currentColor"
        stroke-width=".8"
        opacity=".5"
      />
      <path
        d="M8 46c13-11 25-11 38 0s25 11 46-2"
        stroke="currentColor"
        stroke-width=".7"
        opacity=".32"
      />
      <circle cx="75" cy="27" r="8" stroke="currentColor" stroke-width=".7" opacity=".55" />
    {:else if variant === "orbit"}
      <ellipse
        cx="50"
        cy="50"
        rx="34"
        ry="15"
        stroke="currentColor"
        stroke-width=".9"
        opacity=".62"
        transform="rotate(-24 50 50)"
      />
      <ellipse
        cx="50"
        cy="50"
        rx="27"
        ry="11"
        stroke="currentColor"
        stroke-width=".7"
        opacity=".36"
        transform="rotate(38 50 50)"
      />
      <circle cx="50" cy="50" r="7" stroke="currentColor" stroke-width="1" opacity=".8" />
      <circle cx="50" cy="50" r="2" fill="currentColor" stroke="none" />
      <circle cx="77" cy="38" r="2" fill="currentColor" stroke="none" opacity=".75" />
    {:else if variant === "afterimage"}
      <rect
        x="29"
        y="26"
        width="38"
        height="38"
        stroke="currentColor"
        stroke-width=".8"
        opacity=".26"
      />
      <rect
        x="36"
        y="33"
        width="38"
        height="38"
        stroke="currentColor"
        stroke-width=".9"
        opacity=".44"
      />
      <rect
        x="43"
        y="40"
        width="38"
        height="38"
        stroke="currentColor"
        stroke-width="1"
        opacity=".72"
      />
      <path d="M17 79 79 17" stroke="currentColor" stroke-width=".7" opacity=".3" />
    {:else if variant === "bloom"}
      <g stroke="currentColor" stroke-width=".8" opacity=".58">
        <path d="M50 50c-13-7-13-24 0-31 13 7 13 24 0 31Z" />
        <path d="M50 50c13-7 24 0 27 12-11 10-24 8-27-12Z" />
        <path d="M50 50c13 7 13 24 0 31-13-7-13-24 0-31Z" />
        <path d="M50 50c-13 7-24 0-27-12 11-10 24-8 27 12Z" />
      </g>
      <circle cx="50" cy="50" r="3.2" fill="currentColor" stroke="none" />
    {:else}
      <g stroke="currentColor" stroke-width=".8" opacity=".55">
        <rect x="18" y="24" width="13" height="10" rx="1" />
        <rect x="36" y="24" width="9" height="10" rx="1" />
        <rect x="50" y="24" width="15" height="10" rx="1" />
        <rect x="70" y="24" width="11" height="10" rx="1" />
        <rect x="18" y="42" width="9" height="13" rx="1" />
        <rect x="32" y="42" width="14" height="13" rx="1" />
        <rect x="51" y="42" width="8" height="13" rx="1" />
        <rect x="64" y="42" width="17" height="13" rx="1" />
        <rect x="18" y="63" width="15" height="9" rx="1" />
        <rect x="38" y="63" width="10" height="9" rx="1" />
        <rect x="53" y="63" width="12" height="9" rx="1" />
        <rect x="70" y="63" width="11" height="9" rx="1" />
      </g>
      <path d="M12 84 88 16" stroke="currentColor" stroke-width="1.1" opacity=".45" />
    {/if}
  </svg>

  <span class="cover-art__grain" aria-hidden="true"></span>
</div>

<style>
  .cover-art {
    position: relative;
    width: var(--art-size);
    height: var(--art-size);
    flex: 0 0 auto;
    overflow: hidden;
    border: 1px solid rgb(255 255 255 / 15%);
    border-radius: 7px;
    background: #0d1117;
    box-shadow:
      0 10px 24px rgb(0 0 0 / 42%),
      0 1px 0 rgb(255 255 255 / 16%) inset,
      0 0 0 3px rgb(255 255 255 / 2.5%);
    isolation: isolate;
  }

  .cover-art__wash {
    position: absolute;
    inset: 0;
    background:
      radial-gradient(90% 70% at 22% 12%, rgb(255 255 255 / 16%), transparent 55%),
      linear-gradient(148deg, rgb(255 255 255 / 9%), transparent 42% 68%, rgb(0 0 0 / 24%));
  }

  .cover-art__glyph {
    position: absolute;
    inset: 9%;
    width: 82%;
    height: 82%;
    color: rgb(255 255 255 / 58%);
    filter: drop-shadow(0 0 8px rgb(255 255 255 / 12%));
  }

  .cover-art__grain {
    position: absolute;
    inset: 0;
    background-image: url("data:image/svg+xml,%3Csvg xmlns='http://www.w3.org/2000/svg' width='80' height='80'%3E%3Cfilter id='n'%3E%3CfeTurbulence type='fractalNoise' baseFrequency='.9' numOctaves='2'/%3E%3C/filter%3E%3Crect width='80' height='80' filter='url(%23n)' opacity='.5'/%3E%3C/svg%3E");
    background-size: 80px 80px;
    opacity: 0.13;
    mix-blend-mode: overlay;
    pointer-events: none;
  }

  .cover-art::after {
    content: "";
    position: absolute;
    inset: 0;
    border-radius: inherit;
    box-shadow:
      0 0 0 1px rgb(255 255 255 / 5%) inset,
      0 -18px 28px rgb(0 0 0 / 20%) inset;
    pointer-events: none;
  }

  .cover-art--meridian {
    background:
      radial-gradient(80% 60% at 74% 18%, rgb(94 240 255 / 42%), transparent 55%),
      linear-gradient(158deg, #07171a 6%, #0a343b 48%, #041013 100%);
  }

  .cover-art--meridian .cover-art__glyph {
    color: rgb(180 250 255 / 72%);
  }

  .cover-art--tides {
    background:
      radial-gradient(90% 70% at 24% 14%, rgb(120 210 255 / 34%), transparent 58%),
      linear-gradient(152deg, #07101d 4%, #12334f 48%, #07121d 100%);
  }

  .cover-art--tides .cover-art__glyph {
    color: rgb(158 220 255 / 70%);
  }

  .cover-art--orbit {
    background:
      radial-gradient(80% 70% at 72% 22%, rgb(220 140 255 / 32%), transparent 55%),
      linear-gradient(150deg, #120a24 4%, #2b1747 52%, #0a0614 100%);
  }

  .cover-art--orbit .cover-art__glyph {
    color: rgb(232 196 255 / 72%);
  }

  .cover-art--afterimage {
    background:
      radial-gradient(90% 80% at 80% 18%, rgb(255 170 110 / 48%), transparent 55%),
      linear-gradient(144deg, #260a14 4%, #68243a 52%, #17080d 100%);
  }

  .cover-art--afterimage .cover-art__glyph {
    color: rgb(255 215 190 / 70%);
  }

  .cover-art--bloom {
    background:
      radial-gradient(60% 55% at 50% 46%, rgb(220 255 160 / 52%), transparent 60%),
      radial-gradient(80% 70% at 18% 12%, rgb(94 255 200 / 22%), transparent 55%),
      linear-gradient(150deg, #07130a 4%, #184626 50%, #061008 100%);
  }

  .cover-art--bloom .cover-art__glyph {
    color: rgb(226 255 200 / 75%);
  }

  .cover-art--nightbus {
    background:
      radial-gradient(80% 70% at 78% 16%, rgb(255 196 100 / 46%), transparent 55%),
      linear-gradient(158deg, #170d06 4%, #4a2c12 52%, #0c0704 100%);
  }

  .cover-art--nightbus .cover-art__glyph {
    color: rgb(255 224 168 / 72%);
  }
</style>
