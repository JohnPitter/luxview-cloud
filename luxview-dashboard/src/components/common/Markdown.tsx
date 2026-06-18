import ReactMarkdown from 'react-markdown';
import remarkGfm from 'remark-gfm';
import rehypeHighlight from 'rehype-highlight';
import 'highlight.js/styles/github-dark.css';
import './markdown.css';

interface MarkdownProps {
  children: string;
  className?: string;
}

// Markdown renders user content (READMEs, issue/PR bodies, comments) as GitHub-flavored
// markdown with syntax-highlighted code blocks. Raw HTML is escaped by react-markdown,
// so untrusted input is safe to render.
export function Markdown({ children, className }: MarkdownProps) {
  return (
    <div className={`lv-markdown ${className ?? ''}`}>
      <ReactMarkdown
        remarkPlugins={[remarkGfm]}
        rehypePlugins={[rehypeHighlight]}
        components={{
          a: ({ ...props }) => <a {...props} target="_blank" rel="noreferrer noopener" />,
        }}
      >
        {children}
      </ReactMarkdown>
    </div>
  );
}
