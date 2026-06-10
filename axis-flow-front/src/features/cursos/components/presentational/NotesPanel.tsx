import { useEffect, useRef, useState } from 'react'
import { useNota, useUpsertNota } from '../../hooks/useCursos'

interface NotesPanelProps {
  leccionId: number
}

const IDB_DB_NAME = 'cursos-notes'
const IDB_STORE = 'drafts'
const DEBOUNCE_MS = 500

function openIDB(): Promise<IDBDatabase> {
  return new Promise((resolve, reject) => {
    const req = indexedDB.open(IDB_DB_NAME, 1)
    req.onupgradeneeded = () => req.result.createObjectStore(IDB_STORE)
    req.onsuccess = () => resolve(req.result)
    req.onerror = () => reject(req.error)
  })
}

async function saveToIDB(key: number, value: string): Promise<void> {
  const db = await openIDB()
  return new Promise((resolve, reject) => {
    const tx = db.transaction(IDB_STORE, 'readwrite')
    tx.objectStore(IDB_STORE).put(value, key)
    tx.oncomplete = () => resolve()
    tx.onerror = () => reject(tx.error)
  })
}

async function getFromIDB(key: number): Promise<string | undefined> {
  const db = await openIDB()
  return new Promise((resolve, reject) => {
    const tx = db.transaction(IDB_STORE, 'readonly')
    const req = tx.objectStore(IDB_STORE).get(key)
    req.onsuccess = () => resolve(req.result as string | undefined)
    req.onerror = () => reject(req.error)
  })
}

export function NotesPanel({ leccionId }: NotesPanelProps) {
  const { data: nota } = useNota(leccionId)
  const { mutate: upsertNota } = useUpsertNota()
  const [text, setText] = useState('')
  const [saved, setSaved] = useState(false)
  const debounceRef = useRef<ReturnType<typeof setTimeout> | null>(null)
  const initializedRef = useRef(false)

  // Seed text from server note (once per leccion)
  useEffect(() => {
    if (initializedRef.current) return
    const init = async () => {
      const draft = await getFromIDB(leccionId)
      if (draft !== undefined) {
        setText(draft)
      } else if (nota?.contenido) {
        setText(nota.contenido)
      }
      initializedRef.current = true
    }
    init()
  }, [leccionId, nota])

  // Reset when leccion changes
  useEffect(() => {
    return () => {
      initializedRef.current = false
    }
  }, [leccionId])

  const handleChange = (value: string) => {
    setText(value)
    setSaved(false)

    if (debounceRef.current) clearTimeout(debounceRef.current)
    debounceRef.current = setTimeout(async () => {
      // 1. Save to IndexedDB first
      await saveToIDB(leccionId, value)
      // 2. Then sync to API
      upsertNota(
        { leccionId, contenido: value },
        {
          onSuccess: () => setSaved(true),
        },
      )
    }, DEBOUNCE_MS)
  }

  return (
    <div className="flex flex-col gap-2">
      <div className="flex items-center justify-between">
        <span className="text-sm font-medium text-gray-700">Notes</span>
        {saved && (
          <span className="text-xs text-green-600">Saved</span>
        )}
      </div>
      <textarea
        value={text}
        onChange={(e) => handleChange(e.target.value)}
        className="h-32 w-full resize-none rounded-md border border-gray-300 p-2 text-sm text-gray-800 placeholder-gray-400 focus:outline-none focus:ring-2 focus:ring-blue-500"
        placeholder="Write your notes here…"
        aria-label="Lesson notes"
      />
    </div>
  )
}
