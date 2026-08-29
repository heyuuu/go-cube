// 行内字符级 diff：对 diff 视图的 mod 行（左删右增配对）拆出两侧各自的
// 「公共片段 / 独有片段」序列，前端只对独有片段做高亮，公共片段保持原字体色。
// 超长行退化为整行 changed，避免 LCS 的 O(n·m) 在极端行长下卡顿。

export type InlineSegment = { text: string; changed: boolean };

export type InlineDiff = { left: InlineSegment[]; right: InlineSegment[] };

const MAX_DP_LEN = 1000;

export function splitInlineDiff(oldText: string, newText: string): InlineDiff {
  if (oldText === newText) {
    return { left: [{ text: oldText, changed: false }], right: [{ text: newText, changed: false }] };
  }
  if (oldText.length > MAX_DP_LEN || newText.length > MAX_DP_LEN) {
    return { left: [{ text: oldText, changed: true }], right: [{ text: newText, changed: true }] };
  }

  const a = Array.from(oldText);
  const b = Array.from(newText);
  // LCS 长度表，回溯时同一路径产出两侧片段，天然对齐
  const dp: number[][] = Array.from({ length: a.length + 1 }, () => new Array<number>(b.length + 1).fill(0));
  for (let i = a.length - 1; i >= 0; i--) {
    for (let j = b.length - 1; j >= 0; j--) {
      dp[i][j] = a[i] === b[j] ? dp[i + 1][j + 1] + 1 : Math.max(dp[i + 1][j], dp[i][j + 1]);
    }
  }

  const left: InlineSegment[] = [];
  const right: InlineSegment[] = [];
  let i = 0;
  let j = 0;
  while (i < a.length && j < b.length) {
    if (a[i] === b[j]) {
      push(left, a[i], false);
      push(right, b[j], false);
      i++;
      j++;
    } else if (dp[i + 1][j] >= dp[i][j + 1]) {
      push(left, a[i], true);
      i++;
    } else {
      push(right, b[j], true);
      j++;
    }
  }
  while (i < a.length) push(left, a[i++], true);
  while (j < b.length) push(right, b[j++], true);
  return { left, right };
}

function push(segs: InlineSegment[], text: string, changed: boolean) {
  const last = segs[segs.length - 1];
  if (last && last.changed === changed) last.text += text;
  else segs.push({ text, changed });
}
