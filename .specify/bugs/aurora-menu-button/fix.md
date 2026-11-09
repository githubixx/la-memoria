# Bug Fix: Aurora Glass menu button glint

- **Slug**: aurora-menu-button
- **Fixed**: 2026-10-30
- **Assessment**: ./assessment.md
- **Status**: not-applied

## Summary

No application fix was applied. The user's desired symmetric top and bottom
conflicts with a visible, asymmetric glint also present in Chroma UI's own
burger component. A Bookmarker-specific cause of the reported appearance was
not demonstrated.

## Changes

| File | Change | Notes |
|------|--------|-------|
| `.specify/bugs/aurora-menu-button/fix.md` | added | Investigation and reproducible headless evidence only |

Application CSS, templates, tests, and upstream files were not changed.
Existing worktree changes were left in place.

## Findings

- The pinned dependency is `github.com/githubixx/chroma-ui v0.2.0`
  (`h1:OfbgDt1T9TCnyv9mVrmSLyZvlRwzejTeqEVdTE4Pyss=`). Its
  `templates/header.html` renders the burger as
  `summary.btn.btn-secondary.btn-menu.nav-burger-button`. Thus the upstream
  reference itself opts its burger into `.btn`, not just Bookmarker.
- Aurora's `theme.css` applies `.btn::after` to all these controls: the
  bundled `button-glint.svg` is stretched to `100% 100%`, screen blended,
  at opacity `0.88`. The SVG contains a diagonal sweep, spark, and a
  horizontal stroke near the top; it has no corresponding bottom stroke.
  Aurora also applies a separate inset top highlight and border/shadows.
- Bookmarker uses `button.menu-toggle.btn`, not the upstream summary, and
  its own CSS makes it approximately square with an 8px radius and a plain
  background; upstream's natural reference is a roughly 39 × 25px pill
  with a secondary gradient. These differences can change the effect's
  visibility, but do not by themselves prove an integration defect.
- In isolated headless Chromium, with the pinned CSS and assets and
  Bookmarker's stylesheet order, the glint loaded for all three cases.
  Hiding only `.btn::after` changed the following pixels in the top, middle,
  and bottom screenshot thirds, respectively (maximum RGB-channel change
  greater than 3):

  | Control | Bounding box (CSS px) | Screenshot (px) | Changed pixels, top / middle / bottom |
  |---------|-----------------------|-----------------|---------------------------------------|
  | Upstream native burger | 39.1875 × 25.1875 | 40 × 26 | 191 / 206 / 81 |
  | Upstream burger forced to 45 × 45 | 45 × 45 | 45 × 45 | 306 / 479 / 239 |
  | Bookmarker menu | 45 × 45 | 45 × 45 | 476 / 395 / 309 |

  The forced-square case is an experimental intervention, not upstream's
  default. Pixel counts establish that the decoration affects each region;
  they do not prove that the visual appearance is identical between cases.
- **Confidence:** high that the asymmetric top glint is an intentional
  upstream theme effect and is rendered in both components; medium that it
  is the exact feature the screenshot's reporter objects to. Low confidence
  that any additional Bookmarker-specific defect is responsible. The
  supplied screenshot was inspected, but no pixel-exact match to its
  unknown browser, device scale, viewport, or interaction state was made.

## Tests Added or Updated

None. The user's condition prohibits adding an application regression test
without a demonstrated Bookmarker integration defect.

## Local Verification

- `go list -m -json github.com/githubixx/chroma-ui` → confirmed cached
  v0.2.0 and its module checksum.
- Headless Playwright 1.62.1 / Chromium comparison below → completed
  successfully, with the three results listed above. External requests,
  including the font import, were blocked; fixture CSS and image assets
  came from local files. Transitions and animations were disabled.
- Manual checks: read the pinned upstream component template, base CSS,
  Aurora CSS and SVG, Bookmarker template and styles, and supplied
  screenshot. No visible browser or user's browser was used.
- This was an isolated reproduction, not a running Bookmarker instance;
  no full application suite, keyboard/focus/open-state test, alternate
  browser, or visual approval was performed.

### Reproduce the headless comparison

Run from the repository root with the installed Playwright browser and pinned
Go module available. The script locates the module using `go list`; it does
not write screenshots or change application files.

```sh
node -e '
const {chromium}=require("playwright"), {PNG}=require("playwright-core/lib/utilsBundle"), fs=require("fs");
const moduleDir=require("child_process").execFileSync("go",["list","-m","-f","{{.Dir}}","github.com/githubixx/chroma-ui"],{encoding:"utf8"}).trim();
if(!moduleDir)throw new Error("Chroma UI module directory not found; download the Go module first");
const root=require("path").join(moduleDir,"assets"), app=process.cwd();
(async()=>{const browser=await chromium.launch({headless:true});
for(const kind of ["upstream-native","upstream-square","bookmarker"]){
 const page=await browser.newPage({viewport:{width:800,height:600},deviceScaleFactor:1}); let glint=0;
 await page.route("**/*",route=>{const u=new URL(route.request().url());if(u.hostname!=="fixture.local")return route.abort();const file=u.pathname.startsWith("/ui/")?root+u.pathname.slice(3):u.pathname.startsWith("/assets/")?app+"/web/static"+u.pathname.slice(7):"";if(!file||!fs.existsSync(file)||!fs.statSync(file).isFile())return route.abort();if(u.pathname.endsWith("button-glint.svg"))glint++;return route.fulfill({path:file});});
 const b=kind==="bookmarker";
 const css=`<link rel="stylesheet" href="http://fixture.local/ui/css/base.css">${b?`<link rel="stylesheet" href="http://fixture.local/assets/css/bookmarker.css">`:""}<link rel="stylesheet" href="http://fixture.local/ui/themes/aurora-glass-1/theme.css">${b?`<link rel="stylesheet" href="http://fixture.local/assets/css/chroma-compat.css">`:""}<style>*,*::before,*::after{transition:none!important;animation:none!important}${kind==="upstream-square"?`.nav-burger-button{box-sizing:border-box;width:45px;height:45px}`:""}</style>`;
 const markup=b?`<header class="site-header"><span class="site-title nav-brand">Bookmarker</span><nav class="site-menu menu-ready"><button type="button" class="menu-toggle btn" aria-label="Menu" aria-expanded="false"><span class="menu-icon" aria-hidden="true"><span></span><span></span><span></span></span></button></nav></header>`:`<header><nav><div class="nav-right"><details class="nav-burger"><summary class="btn btn-secondary btn-menu nav-burger-button" aria-label="Menu"><span class="burger-icon" aria-hidden="true"></span></summary></details></div></nav></header>`;
 await page.setContent(`<html data-theme="aurora-glass-1"><head>${css}</head><body>${markup}</body></html>`,{waitUntil:"load"});await page.mouse.move(0,599);
 const button=page.locator(b?".menu-toggle":".nav-burger-button");
 const computed=await button.evaluate(e=>{const s=getComputedStyle(e),a=getComputedStyle(e,"::after"),r=e.getBoundingClientRect();return {size:[r.width,r.height],background:s.backgroundImage,radius:s.borderRadius,glint:a.backgroundImage,scale:a.backgroundSize,blend:a.mixBlendMode,opacity:a.opacity}});
 const on=PNG.sync.read(await button.screenshot());await page.addStyleTag({content:":root[data-theme=aurora-glass-1] .btn::after{opacity:0!important}"});const off=PNG.sync.read(await button.screenshot());
 let bands=[0,0,0];for(let y=0;y<on.height;y++)for(let x=0;x<on.width;x++){let i=(y*on.width+x)*4;if(Math.max(...[0,1,2].map(c=>Math.abs(on.data[i+c]-off.data[i+c])))>3)bands[Math.min(2,Math.floor(y*3/on.height))]++;}
 console.log(JSON.stringify({kind,computed,glintRequests:glint,screenshot:[on.width,on.height],changedPixelsTopMiddleBottom:bands}));await page.close();}
 await browser.close();})().catch(e=>{console.error(e);process.exit(1)});'
```

## Deviations from Assessment

- The assessment proposed suppressing or replacing the menu glint after
  confirming the desired appearance. The user clarified the desired
  top-versus-bottom appearance and explicitly excluded fixes when the
  origin is upstream or uncertain. The upstream reference and controlled
  rendering do not establish an integration-specific defect, so the
  proposed application change and corresponding tests were not performed.
- The assessment's environment questions remain unresolved. The comparison
  is useful for source attribution, not proof of a pixel-perfect match in
  the reporter's environment.

## Follow-ups

- If desired, discuss the asymmetric glint as an upstream Chroma design
  question. Do not change upstream or Bookmarker without an explicit decision.
- `/speckit.bug.test slug=aurora-menu-button` can record independent
  verification of this finding; there is no applied fix to validate.
