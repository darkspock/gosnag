import { useEditor, EditorContent, type Editor } from '@tiptap/react'
import StarterKit from '@tiptap/starter-kit'
import Image from '@tiptap/extension-image'
import Link from '@tiptap/extension-link'
import Placeholder from '@tiptap/extension-placeholder'
import { useCallback, useEffect } from 'react'
import { Bold, Italic, Code, List, ListOrdered, Link as LinkIcon, Image as ImageIcon, Heading2, Quote, Minus } from 'lucide-react'
import { cn } from '@/lib/utils'

interface RichEditorProps {
  content: string
  onChange: (html: string) => void
  placeholder?: string
  className?: string
  editable?: boolean
  onImageUpload?: (file: File) => Promise<string>
}

function ToolbarButton({ onClick, active, children, title }: { onClick: () => void; active?: boolean; children: React.ReactNode; title: string }) {
  return (
    <button
      type="button"
      title={title}
      onClick={onClick}
      className={cn(
        'h-7 w-7 flex items-center justify-center rounded transition-colors',
        active ? 'bg-primary/20 text-primary' : 'text-muted-foreground hover:text-foreground hover:bg-accent'
      )}
    >
      {children}
    </button>
  )
}

function Toolbar({ editor, onImageUpload }: { editor: Editor; onImageUpload?: (file: File) => Promise<string> }) {
  const addLink = useCallback(() => {
    const url = prompt('URL:')
    if (url) {
      editor.chain().focus().extendMarkRange('link').setLink({ href: url }).run()
    }
  }, [editor])

  const addImage = useCallback(async () => {
    if (onImageUpload) {
      const input = document.createElement('input')
      input.type = 'file'
      input.accept = 'image/*'
      input.onchange = async () => {
        const file = input.files?.[0]
        if (file) {
          const url = await onImageUpload(file)
          editor.chain().focus().setImage({ src: url }).run()
        }
      }
      input.click()
    } else {
      const url = prompt('Image URL:')
      if (url) {
        editor.chain().focus().setImage({ src: url }).run()
      }
    }
  }, [editor, onImageUpload])

  return (
    <div className="flex items-center gap-0.5 px-2 py-1 border-b bg-muted/30 flex-wrap">
      <ToolbarButton title="Bold" onClick={() => editor.chain().focus().toggleBold().run()} active={editor.isActive('bold')}>
        <Bold className="h-3.5 w-3.5" />
      </ToolbarButton>
      <ToolbarButton title="Italic" onClick={() => editor.chain().focus().toggleItalic().run()} active={editor.isActive('italic')}>
        <Italic className="h-3.5 w-3.5" />
      </ToolbarButton>
      <ToolbarButton title="Code" onClick={() => editor.chain().focus().toggleCode().run()} active={editor.isActive('code')}>
        <Code className="h-3.5 w-3.5" />
      </ToolbarButton>
      <div className="w-px h-4 bg-border mx-1" />
      <ToolbarButton title="Heading" onClick={() => editor.chain().focus().toggleHeading({ level: 3 }).run()} active={editor.isActive('heading')}>
        <Heading2 className="h-3.5 w-3.5" />
      </ToolbarButton>
      <ToolbarButton title="Bullet list" onClick={() => editor.chain().focus().toggleBulletList().run()} active={editor.isActive('bulletList')}>
        <List className="h-3.5 w-3.5" />
      </ToolbarButton>
      <ToolbarButton title="Ordered list" onClick={() => editor.chain().focus().toggleOrderedList().run()} active={editor.isActive('orderedList')}>
        <ListOrdered className="h-3.5 w-3.5" />
      </ToolbarButton>
      <ToolbarButton title="Blockquote" onClick={() => editor.chain().focus().toggleBlockquote().run()} active={editor.isActive('blockquote')}>
        <Quote className="h-3.5 w-3.5" />
      </ToolbarButton>
      <ToolbarButton title="Code block" onClick={() => editor.chain().focus().toggleCodeBlock().run()} active={editor.isActive('codeBlock')}>
        <Minus className="h-3.5 w-3.5" />
      </ToolbarButton>
      <div className="w-px h-4 bg-border mx-1" />
      <ToolbarButton title="Link" onClick={addLink} active={editor.isActive('link')}>
        <LinkIcon className="h-3.5 w-3.5" />
      </ToolbarButton>
      <ToolbarButton title="Image" onClick={addImage}>
        <ImageIcon className="h-3.5 w-3.5" />
      </ToolbarButton>
    </div>
  )
}

export function RichEditor({ content, onChange, placeholder, className, editable = true, onImageUpload }: RichEditorProps) {
  const editor = useEditor({
    extensions: [
      StarterKit.configure({
        heading: { levels: [2, 3] },
      }),
      Image.configure({
        allowBase64: false,
        HTMLAttributes: { class: 'rounded-md max-w-full' },
      }),
      Link.configure({
        openOnClick: true,
        HTMLAttributes: { class: 'text-primary underline' },
      }),
      Placeholder.configure({
        placeholder: placeholder || 'Start writing...',
      }),
    ],
    content,
    editable,
    onUpdate: ({ editor }) => {
      onChange(editor.getHTML())
    },
    editorProps: {
      attributes: {
        class: 'prose prose-sm prose-invert max-w-none focus:outline-none min-h-[100px] px-3 py-2 [&_pre]:bg-muted [&_pre]:p-2 [&_pre]:rounded [&_code]:text-xs [&_p]:my-1.5 [&_img]:rounded-md [&_img]:max-w-full [&_h2]:text-base [&_h2]:font-semibold [&_h3]:text-sm [&_h3]:font-semibold [&_blockquote]:border-l-2 [&_blockquote]:border-primary/30 [&_blockquote]:pl-3 [&_blockquote]:italic [&_blockquote]:text-muted-foreground',
      },
      handlePaste: (_view, event) => {
        const items = event.clipboardData?.items
        if (!items || !onImageUpload) return false

        for (const item of items) {
          if (item.type.startsWith('image/')) {
            event.preventDefault()
            const file = item.getAsFile()
            if (file) {
              onImageUpload(file).then(url => {
                editor?.chain().focus().setImage({ src: url }).run()
              })
            }
            return true
          }
        }
        return false
      },
      handleDrop: (_view, event) => {
        const files = event.dataTransfer?.files
        if (!files?.length || !onImageUpload) return false

        for (const file of files) {
          if (file.type.startsWith('image/')) {
            event.preventDefault()
            onImageUpload(file).then(url => {
              editor?.chain().focus().setImage({ src: url }).run()
            })
            return true
          }
        }
        return false
      },
    },
  })

  useEffect(() => {
    if (editor && !editable) {
      editor.commands.setContent(content)
    }
  }, [content, editor, editable])

  if (!editor) return null

  return (
    <div className={cn('rounded-md border bg-card overflow-hidden', className)}>
      {editable && <Toolbar editor={editor} onImageUpload={onImageUpload} />}
      <EditorContent editor={editor} />
    </div>
  )
}

export function RichViewer({ content, className }: { content: string; className?: string }) {
  const editor = useEditor({
    extensions: [
      StarterKit.configure({ heading: { levels: [2, 3] } }),
      Image.configure({ HTMLAttributes: { class: 'rounded-md max-w-full' } }),
      Link.configure({ openOnClick: true, HTMLAttributes: { class: 'text-primary underline' } }),
    ],
    content,
    editable: false,
  })

  useEffect(() => {
    if (editor) editor.commands.setContent(content)
  }, [content, editor])

  if (!editor) return null

  return (
    <div className={cn('prose prose-sm prose-invert max-w-none [&_pre]:bg-muted [&_pre]:p-2 [&_pre]:rounded [&_code]:text-xs [&_p]:my-1.5 [&_img]:rounded-md [&_img]:max-w-full [&_blockquote]:border-l-2 [&_blockquote]:border-primary/30 [&_blockquote]:pl-3', className)}>
      <EditorContent editor={editor} />
    </div>
  )
}
