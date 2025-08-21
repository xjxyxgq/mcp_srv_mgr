declare module 'monaco-editor' {
  export interface IEditorOptions {
    minimap?: { enabled: boolean };
    fontSize?: number;
    lineNumbers?: 'on' | 'off';
    roundedSelection?: boolean;
    scrollBeyondLastLine?: boolean;
    readOnly?: boolean;
    automaticLayout?: boolean;
    'editor.background'?: string;
    'editor.foreground'?: string;
    'editor.lineHighlightBackground'?: string;
    'editor.selectionBackground'?: string;
    'editor.inactiveSelectionBackground'?: string;
    'editor.lineHighlightBorder'?: string;
    'editorCursor.foreground'?: string;
    'editorWhitespace.foreground'?: string;
  }
}

declare global {
  interface Window {
    monaco: typeof import('monaco-editor');
  }
} 