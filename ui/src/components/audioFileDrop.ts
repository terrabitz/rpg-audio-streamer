const audioExtensions = ['.mp3', '.wav', '.aac', '.m4a', '.flac', '.ogg', '.oga', '.opus', '.webm', '.wma', '.aif', '.aiff', '.m4b', '.alac']
export const audioFileAccept = ['audio/*', ...audioExtensions].join(',')

export async function collectDroppedAudio(transfer: DataTransfer) {
  const result = { files: [] as File[], unreadable: 0, unsupported: 0 }
  // Capture items before the browser clears access to the drop's data.
  const items = Array.from(transfer.items).filter(item => item.kind === 'file').map(item => {
    const entry = item.webkitGetAsEntry?.()
    return { entry, file: entry ? null : item.getAsFile() }
  })
  if (!items.length) {
    for (const file of Array.from(transfer.files)) acceptFile(file)
    return result
  }
  for (const { entry, file } of items) {
    if (entry) await readEntry(entry)
    else if (file) acceptFile(file)
    else result.unreadable++
  }
  return result

  async function readEntry(entry: FileSystemEntry): Promise<void> {
    try {
      if (isDirectory(entry)) {
        const reader = entry.createReader()
        // A reader can return only a subset per call, even for one directory.
        for (;;) {
          const children = await new Promise<FileSystemEntry[]>((resolve, reject) => reader.readEntries(resolve, reject))
          if (!children.length) break
          for (const child of children) await readEntry(child)
        }
      } else if (isFile(entry)) {
        acceptFile(await new Promise<File>((resolve, reject) => entry.file(resolve, reject)))
      }
    } catch (error) {
      console.warn('Unable to read dropped item:', entry.name, error)
      result.unreadable++
    }
  }

  function acceptFile(file: File) {
    const extension = file.name.slice(file.name.lastIndexOf('.')).toLowerCase()
    if (file.size > 0 && (file.type.startsWith('audio/') || audioExtensions.includes(extension))) {
      result.files.push(file)
    } else {
      result.unsupported++
    }
  }
}

function isDirectory(entry: FileSystemEntry): entry is FileSystemDirectoryEntry {
  return entry.isDirectory
}

function isFile(entry: FileSystemEntry): entry is FileSystemFileEntry {
  return entry.isFile
}
