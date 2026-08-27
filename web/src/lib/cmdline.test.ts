import { describe, expect, it } from 'vitest';

import { joinCmdLine, tokenizeCmdLine } from './cmdline';

describe('tokenizeCmdLine', () => {
  it('空白分隔多个 token', () => {
    expect(tokenizeCmdLine('code $0')).toEqual(['code', '$0']);
  });

  it('双引号内的空格不切分（典型：.app 路径）', () => {
    expect(tokenizeCmdLine('"/Applications/Visual Studio Code.app/Contents/Resources/app/bin/code" $0')).toEqual([
      '/Applications/Visual Studio Code.app/Contents/Resources/app/bin/code',
      '$0',
    ]);
  });

  it('单引号同样保留空格', () => {
    expect(tokenizeCmdLine("'a b' c")).toEqual(['a b', 'c']);
  });

  it('双引号内 \\" 转义为字面引号', () => {
    expect(tokenizeCmdLine('"say \\"hi\\"" x')).toEqual(['say "hi"', 'x']);
  });

  it('多余空白与空输入', () => {
    expect(tokenizeCmdLine('  a   b  ')).toEqual(['a', 'b']);
    expect(tokenizeCmdLine('')).toEqual([]);
  });

  it('未闭合引号原样收尾（容错）', () => {
    expect(tokenizeCmdLine('"abc def')).toEqual(['abc def']);
  });
});

describe('joinCmdLine', () => {
  it('普通 token 原样拼接', () => {
    expect(joinCmdLine(['code', '$0'])).toBe('code $0');
  });

  it('含空格的 token 加双引号，与 tokenize 互逆', () => {
    const tokens = ['/Applications/Visual Studio Code.app/.../bin/code', '$0'];
    expect(tokenizeCmdLine(joinCmdLine(tokens))).toEqual(tokens);
  });

  it('含双引号的 token 转义后仍互逆', () => {
    const tokens = ['a "b" c', 'x'];
    expect(tokenizeCmdLine(joinCmdLine(tokens))).toEqual(tokens);
  });
});
