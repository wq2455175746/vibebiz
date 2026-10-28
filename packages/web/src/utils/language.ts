const LANGUAGE_BY_EXTENSION: Record<string, string> = {
  html: 'html',
  htm: 'html',
  css: 'css',
  scss: 'scss',
  less: 'less',
  js: 'javascript',
  mjs: 'javascript',
  cjs: 'javascript',
  ts: 'typescript',
  tsx: 'typescript',
  jsx: 'javascript',
  json: 'json',
  md: 'markdown',
  py: 'python',
  go: 'go',
  yml: 'yaml',
  yaml: 'yaml',
  sh: 'shell',
  svg: 'xml',
  xml: 'xml',
  txt: 'plaintext',
}

export function detectLanguage(path: string): string {
  const extension = path.split('.').pop()?.toLowerCase() ?? ''
  return LANGUAGE_BY_EXTENSION[extension] ?? 'plaintext'
}
