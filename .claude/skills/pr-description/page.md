Turn the pull request description on stdin into a one-page HTML summary for a person
reviewing the PR. Print only the HTML document, starting with `<!doctype html>`, with no
Markdown fence and no preamble.

The description is the only source. Restate it; never add a fact, a test, a risk or a
judgement that isn't in it, and never drop a failed check, an untested item or a risk.
If it says something is not met or not run, the page says so just as plainly.

Write for a person, not an agent:
- Plain language and short sentences. Keep a file path, command or test name only where
  the reader needs it to act or check; otherwise describe what it is.
- Put the verdict first: can this merge, and if not, what blocks it.
- Count the conformance statuses for the tally from the description's table exactly.

Use this template. Keep its `<head>` and CSS unchanged. Fill each `<!-- … -->` slot,
repeat the example elements as needed, and drop a section only when the description has
nothing for it. Pill classes: `ok` (met, passed), `warn` (untested, partial, a risk to
watch), `bad` (not met, failed, a blocking or destructive risk). Use `verdict ok` and a
"Ready to merge" heading only when nothing is `bad`.

```html
<!doctype html>
<html lang="en">
<head>
<meta charset="utf-8">
<meta name="viewport" content="width=device-width, initial-scale=1">
<title><!-- 2–4 word name of the change --> Review</title>
<link rel="stylesheet" href="https://fonts.googleapis.com/css2?family=Bricolage+Grotesque:opsz,wght@12..96,600;12..96,700&family=IBM+Plex+Sans:wght@400;500;600&family=IBM+Plex+Mono:wght@400;500&display=swap">
<style>
:root {
  --bg: #f6f7f5; --surface: #ffffff; --fg: #1d2420; --muted: #5d6a63; --line: #dde3df;
  --accent: #1f6f5c;
  --ok: #2c7a3f; --ok-soft: #e3f3e6; --warn: #9a6200; --warn-soft: #fbf0d9; --bad: #b3261e; --bad-soft: #fbe5e3;
  --code-bg: #eef1ef;
  --display: "Bricolage Grotesque", "Segoe UI", system-ui, sans-serif;
  --body: "IBM Plex Sans", "Segoe UI", system-ui, sans-serif;
  --mono: "IBM Plex Mono", ui-monospace, Menlo, monospace;
  color-scheme: light;
}
@media (prefers-color-scheme: dark) { :root {
  --bg: #121614; --surface: #1a201d; --fg: #e3e9e5; --muted: #98a59e; --line: #2c3530;
  --accent: #6cc4a8;
  --ok: #7fd394; --ok-soft: #1c3022; --warn: #f0bd5c; --warn-soft: #362a12; --bad: #f2867c; --bad-soft: #3a1c1a;
  --code-bg: #232a26; color-scheme: dark; } }
* { box-sizing: border-box; }
body { margin: 0; background: var(--bg); color: var(--fg); font: 15px/1.6 var(--body); }
.wrap { max-width: 860px; margin: 0 auto; padding: 40px 16px 72px; display: flex; flex-direction: column; gap: 40px; }
h1, h2, h3 { font-family: var(--display); text-wrap: balance; margin: 0; line-height: 1.2; }
h1 { font-size: 2.1rem; } h2 { font-size: 1.35rem; } h3 { font-size: 1rem; }
p { margin: 0; max-width: 68ch; }
code { font-family: var(--mono); font-size: .86em; background: var(--code-bg); padding: 1px 5px; border-radius: 4px; overflow-wrap: anywhere; }
section { display: flex; flex-direction: column; gap: 14px; min-width: 0; }
.eyebrow { font: 500 .75rem/1 var(--mono); letter-spacing: .08em; text-transform: uppercase; color: var(--muted); }
.lede { color: var(--muted); font-size: 1.05rem; }
header { display: flex; flex-direction: column; gap: 12px; }
.meta { display: flex; flex-wrap: wrap; gap: 8px 18px; color: var(--muted); font-size: .88rem; }
.verdict { border-radius: 10px; padding: 18px 20px; display: flex; flex-direction: column; gap: 8px; }
.verdict.bad { background: var(--bad-soft); border: 1px solid var(--bad); }
.verdict.ok { background: var(--ok-soft); border: 1px solid var(--ok); }
.verdict.bad h2 { color: var(--bad); } .verdict.ok h2 { color: var(--ok); }
.verdict h2 { font-size: 1.15rem; }
.tally { display: grid; grid-template-columns: repeat(4, minmax(0, 1fr)); gap: 10px; }
.tally div { background: var(--surface); border: 1px solid var(--line); border-radius: 8px; padding: 12px 14px; }
.tally b { display: block; font: 700 1.6rem/1.1 var(--display); font-variant-numeric: tabular-nums; }
.tally span { color: var(--muted); font-size: .82rem; }
.tally .ok b { color: var(--ok); } .tally .warn b { color: var(--warn); } .tally .bad b { color: var(--bad); }
@media (max-width: 560px) { .tally { grid-template-columns: repeat(2, minmax(0, 1fr)); } }
.modules { display: grid; grid-template-columns: repeat(auto-fit, minmax(240px, 1fr)); gap: 12px; }
.mod { background: var(--surface); border: 1px solid var(--line); border-radius: 8px; padding: 14px 16px; display: flex; flex-direction: column; gap: 6px; min-width: 0; }
.mod h3 { font-family: var(--mono); font-weight: 500; font-size: .9rem; color: var(--accent); }
.mod ul { margin: 0; padding-left: 18px; font-size: .92rem; }
.table-scroll { overflow-x: auto; border: 1px solid var(--line); border-radius: 8px; background: var(--surface); }
table { border-collapse: collapse; width: 100%; font-size: .9rem; }
th, td { text-align: left; padding: 10px 14px; border-bottom: 1px solid var(--line); vertical-align: top; }
th { font: 500 .72rem/1 var(--mono); letter-spacing: .06em; text-transform: uppercase; color: var(--muted); }
tr:last-child td { border-bottom: 0; }
td.ev { color: var(--muted); font-size: .84rem; }
.pill { display: inline-block; white-space: nowrap; font: 500 .75rem/1 var(--mono); padding: 4px 8px; border-radius: 999px; }
.pill.ok { background: var(--ok-soft); color: var(--ok); }
.pill.warn { background: var(--warn-soft); color: var(--warn); }
.pill.bad { background: var(--bad-soft); color: var(--bad); }
.risks { display: flex; flex-direction: column; gap: 10px; }
.risk { display: grid; grid-template-columns: 92px minmax(0, 1fr); gap: 14px; background: var(--surface); border: 1px solid var(--line); border-radius: 8px; padding: 14px 16px; }
.risk p { font-size: .93rem; } .risk strong { display: block; margin-bottom: 2px; }
@media (max-width: 520px) { .risk { grid-template-columns: 1fr; gap: 6px; } }
ol.steps, ul.plain { margin: 0; padding-left: 22px; display: flex; flex-direction: column; gap: 6px; }
.diagram { background: var(--surface); border: 1px solid var(--line); border-radius: 8px; padding: 16px; overflow-x: auto; }
.note { color: var(--muted); font-size: .85rem; }
</style>
</head>
<body>
<div class="wrap">
  <header>
    <span class="eyebrow">Pull request review · <!-- branch --> → <!-- base --></span>
    <h1><!-- what the change does, as a headline --></h1>
    <p class="lede"><!-- the why, in one or two plain sentences --></p>
    <div class="meta">
      <span>Plan: <!-- OpenSpec change, plan document or ticket --></span>
      <span>Reviewed by a separate session that saw only the diff and the plan</span>
    </div>
  </header>

  <div class="verdict bad" role="status">
    <h2><!-- "Not ready to merge yet" or "Ready to merge" --></h2>
    <p><!-- what blocks the merge, or what still needs a person --></p>
  </div>

  <section>
    <span class="eyebrow">Plan vs. what was built</span>
    <div class="tally">
      <div class="ok"><b><!-- n --></b><span>met, with tests</span></div>
      <div class="warn"><b><!-- n --></b><span>met, missing tests</span></div>
      <div class="bad"><b><!-- n --></b><span>not met</span></div>
      <div><b><!-- n / m --></b><span>tasks done</span></div>
    </div>
    <div class="table-scroll">
      <table>
        <thead><tr><th>Plan item</th><th>Status</th><th>Evidence</th></tr></thead>
        <tbody>
          <tr><td><!-- requirement in plain words --></td><td><span class="pill ok">met</span></td><td class="ev"><!-- tests --></td></tr>
        </tbody>
      </table>
    </div>
    <p class="note">Not in the plan: <!-- out-of-plan changes, or "nothing" --></p>
  </section>

  <section>
    <span class="eyebrow">How it fits together</span>
    <div class="diagram"><pre class="mermaid">
<!-- the description's mermaid diagram, labels in plain words; omit the section if it has none -->
    </pre></div>
  </section>

  <section>
    <span class="eyebrow">What changed, by module</span>
    <div class="modules">
      <div class="mod"><h3><!-- module/ --></h3><ul><li><!-- change --></li></ul></div>
    </div>
  </section>

  <section>
    <span class="eyebrow">Risks</span>
    <div class="risks">
      <div class="risk"><span class="pill bad"><!-- one-word kind --></span><p><strong><!-- the risk --></strong><!-- what to do about it --></p></div>
    </div>
  </section>

  <section>
    <span class="eyebrow">How to check it yourself</span>
    <h3>What the reviewer ran</h3>
    <ul class="plain"><li><!-- check and its result --></li></ul>
    <h3>Manual check, not done yet</h3>
    <ol class="steps"><li><!-- step --></li></ol>
  </section>

  <section>
    <span class="eyebrow">Smaller notes</span>
    <ul class="plain"><li><!-- assumption or unverified item --></li></ul>
  </section>
</div>
<script src="https://cdn.jsdelivr.net/npm/mermaid@10.9.1/dist/mermaid.min.js"></script>
<script>mermaid.initialize({ startOnLoad: true, theme: matchMedia("(prefers-color-scheme: dark)").matches ? "dark" : "neutral" });</script>
</body>
</html>
```
