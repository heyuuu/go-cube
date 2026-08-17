/**
 * 只读校验 src/components/ui 下的组件与 shadcn 官方 registry 输出是否一致。
 *
 * 通过 `shadcn add --view` 拿到 registry 的最终文件内容（不落盘），写入临时目录
 * 并用项目 oxfmt 规则格式化归一后，与本地文件逐一比对。全程不修改仓库文件、
 * 不依赖 git 状态，可与其他改动并行执行。
 */
import { spawnSync } from 'node:child_process';
import { mkdirSync, mkdtempSync, readFileSync, readdirSync, rmSync, writeFileSync } from 'node:fs';
import os from 'node:os';
import path from 'node:path';
import process from 'node:process';

const webDir = path.resolve(import.meta.dirname, '..');
const uiDir = path.join(webDir, 'src/components/ui');

function fail(message: string): never {
  console.error(`✗ ${message}`);
  process.exit(1);
}

// 组件清单从 ui 目录扫描得出，后续新增组件无需改本脚本
const components = readdirSync(uiDir)
  .filter((f) => f.endsWith('.tsx'))
  .map((f) => path.basename(f, '.tsx'));
if (components.length === 0) fail('src/components/ui 下没有找到任何组件');

// 逐组件拉取：批跑 --view 时 CLI 会截断大输出，多文件 section 会整段丢失
const registryFiles = new Map<string, string>();
for (const component of components) {
  console.log(`↓ 拉取 ${component} ...`);
  const view = spawnSync('pnpm', ['dlx', 'shadcn@latest', 'add', component, '--view'], {
    cwd: webDir,
    stdio: 'pipe',
    encoding: 'utf8',
  });
  if (view.status !== 0) fail(`拉取 ${component} 失败：\n${view.stderr}`);
  parseRegistrySections(view.stdout, registryFiles);
}

// --view 输出形如（每个文件一段）：
//   ├ src/components/ui/badge.tsx (overwrite) 53 lines
//   │ ┌──────────
//   │ │ <内容行>
//   │ └──────────
function parseRegistrySections(output: string, into: Map<string, string>) {
  let currentPath: string | null = null;
  let currentLines: string[] | null = null;
  for (const line of output.split('\n')) {
    if (line.startsWith('├ ')) {
      const match = line.match(/^├ (\S+)/);
      currentPath = match?.[1] ?? null;
      currentLines = null;
    } else if (line.startsWith('│ ┌')) {
      currentLines = [];
    } else if (line.startsWith('│ └')) {
      if (currentPath && currentLines) {
        into.set(currentPath, currentLines.join('\n') + '\n');
        currentPath = null;
        currentLines = null;
      }
    } else if (currentLines !== null && line.startsWith('│ │')) {
      // 内容行前缀为 "│ │ "，空行前缀为 "│ │"（无尾随空格）
      currentLines.push(line.slice(4));
    }
  }
}

// registry 内容写入临时目录，用项目 oxfmt 规则格式化，
// 消除 registry 内置 prettier 与项目 oxfmt 的纯格式差异
const tmpDir = mkdtempSync(path.join(os.tmpdir(), 'cube-verify-shadcn-'));
const tmpUiDir = path.join(tmpDir, 'src/components/ui');
const uiFiles = [...registryFiles.keys()].filter((p) => p.startsWith('src/components/ui/'));
if (uiFiles.length === 0) fail('--view 输出解析失败，没有提取到任何组件内容（CLI 输出格式可能已变更）');
for (const file of uiFiles) {
  const target = path.join(tmpDir, file);
  mkdirSync(path.dirname(target), { recursive: true });
  writeFileSync(target, registryFiles.get(file)!);
}
const fmt = spawnSync('pnpm', ['exec', 'oxfmt', tmpUiDir], { cwd: webDir, stdio: 'pipe', encoding: 'utf8' });
if (fmt.status !== 0) {
  rmSync(tmpDir, { recursive: true, force: true });
  fail(`oxfmt 格式化临时文件失败：\n${fmt.stderr}`);
}

let mismatched = false;
for (const component of components) {
  const relPath = `src/components/ui/${component}.tsx`;
  const tmpFile = path.join(tmpDir, relPath);
  if (!registryFiles.has(relPath)) {
    console.error(`✗ ${component}: registry 中不存在（本地文件疑似自创，请人工确认）`);
    mismatched = true;
    continue;
  }
  const registry = readFileSync(tmpFile, 'utf8');
  const local = readFileSync(path.join(uiDir, `${component}.tsx`), 'utf8');
  if (registry === local) {
    console.log(`✓ ${component}`);
  } else {
    console.error(`✗ ${component}: 与官方 registry 输出不一致`);
    mismatched = true;
  }
}
// 失败时保留临时目录，供 cube diff 人工比对（成功才清理）
if (!mismatched) rmSync(tmpDir, { recursive: true, force: true });

if (mismatched) {
  console.error(`\n用对比工具查看具体差异：`);
  console.error(`  cube diff ${uiDir} ${tmpUiDir}`);
  console.error(`（${tmpDir} 为 registry 输出副本，确认后可删除）`);
  process.exitCode = 1;
} else {
  console.log(`✓ ${components.length} 个组件全部与官方 registry 输出一致`);
}
