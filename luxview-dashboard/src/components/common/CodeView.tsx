import { useMemo } from 'react';
import hljs from 'highlight.js';
import 'highlight.js/styles/github-dark.css';

const EXT_LANG: Record<string, string> = {
  js: 'javascript', mjs: 'javascript', cjs: 'javascript', jsx: 'javascript',
  ts: 'typescript', tsx: 'typescript', py: 'python', go: 'go', rs: 'rust',
  java: 'java', rb: 'ruby', php: 'php', c: 'c', h: 'c', cpp: 'cpp', cc: 'cpp',
  hpp: 'cpp', cs: 'csharp', json: 'json', yml: 'yaml', yaml: 'yaml',
  md: 'markdown', sh: 'bash', bash: 'bash', zsh: 'bash', sql: 'sql',
  html: 'xml', xml: 'xml', css: 'css', scss: 'scss', less: 'less',
  kt: 'kotlin', swift: 'swift', toml: 'ini', ini: 'ini', dockerfile: 'dockerfile',
};

function langFor(fileName: string): string | undefined {
  const lower = fileName.toLowerCase();
  if (lower === 'dockerfile') return 'dockerfile';
  const ext = lower.includes('.') ? lower.split('.').pop()! : '';
  const lang = EXT_LANG[ext];
  return lang && hljs.getLanguage(lang) ? lang : undefined;
}

interface CodeViewProps {
  content: string;
  fileName: string;
}

// CodeView renders a source file with syntax highlighting and a line-number gutter.
export function CodeView({ content, fileName }: CodeViewProps) {
  const html = useMemo(() => {
    const lang = langFor(fileName);
    try {
      return lang
        ? hljs.highlight(content, { language: lang }).value
        : hljs.highlightAuto(content).value;
    } catch {
      return null;
    }
  }, [content, fileName]);

  const lineCount = content.split('\n').length;

  return (
    <div className="flex text-xs font-mono leading-5 overflow-x-auto">
      <div className="select-none text-right pr-4 pl-3 py-4 text-zinc-600 flex-shrink-0">
        {Array.from({ length: lineCount }, (_, i) => (
          <div key={i}>{i + 1}</div>
        ))}
      </div>
      <pre className="hljs flex-1 py-4 pr-4 !bg-transparent">
        {/* Safe: highlight.js HTML-escapes the source text and only injects its own
            <span> markup, so no untrusted HTML reaches the DOM. */}
        {html !== null
          ? <code dangerouslySetInnerHTML={{ __html: html }} />
          : <code>{content}</code>}
      </pre>
    </div>
  );
}
