import { beforeEach, describe, expect, it } from 'vitest'
import { createPinia, setActivePinia } from 'pinia'
import { useAudioStore } from '../audio'

describe('audio session state', () => {
  beforeEach(() => setActivePinia(createPinia()))

  it('uses the saved table mix without replacing player preferences or restoring playback', () => {
    const audio = useAudioStore()
    audio.masterVolume = 45
    audio.setTypeVolume('Music', 25)
    audio.loadTableMix({
      masterVolume: 0, typeVolumes: { Music: 0 }, trackVolumes: { forest: 0 }, trackOrder: ['forest'],
    })
    audio.initTrack('forest', 'Forest')
    audio.initTrack('rain', 'Rain')
    expect(audio.mixMasterVolume).toBe(0)
    expect(audio.getTypeVolume('Music')).toBe(0)
    expect(audio.tracks.forest).toMatchObject({ volume: 0, isPlaying: false, currentTime: 0 })
    expect(audio.tracks.rain?.volume).toBe(100)
    audio.setMasterVolume(80)
    audio.setTypeVolume('Music', 60)
    audio.updateTrackState('forest', { volume: 30, currentTime: 12, isPlaying: true })
    expect(audio.tableMix).toEqual({
      masterVolume: 80, typeVolumes: { Music: 60 }, trackVolumes: { forest: 30 },
    })
    expect(audio.masterVolume).toBe(45)
    expect(audio.typeVolumes).toEqual({ Music: 25 })
    audio.clearTracks()
    audio.clearTableMix()
    expect(audio.mixMasterVolume).toBe(45)
    expect(audio.getTypeVolume('Music')).toBe(25)
    audio.setMasterVolume(0)
    expect(audio.masterVolume).toBe(0)
    audio.loadTableMix({ masterVolume: 100, typeVolumes: {}, trackVolumes: {}, trackOrder: [] })
    audio.initTrack('forest', 'Forest')
    expect(audio.tracks.forest).toMatchObject({ volume: 100, currentTime: 0, isPlaying: false })
  })

  it('clears table playback and fades while preserving local mix preferences', () => {
    const audio = useAudioStore()
    audio.enabled = true
    audio.masterVolume = 45
    audio.setTypeVolume('Music', 25)
    audio.initTrack('forest', 'Forest')
    audio.updateTrackState('forest', { isPlaying: true })
    audio.setFading('forest', true)
    audio.clearTracks()
    expect(audio.enabled).toBe(false)
    expect(audio.tracks).toEqual({})
    expect(audio.fadeStates).toEqual({})
    expect(audio.masterVolume).toBe(45)
    expect(audio.getTypeVolume('Music')).toBe(25)
  })

  it('initializes stopped tracks without overwriting existing state', () => {
    const audio = useAudioStore()
    audio.initTrack('forest', 'Forest', 'ambiance')
    expect(audio.tracks.forest).toEqual({
      fileID: 'forest',
      name: 'Forest',
      trackType: 'ambiance',
      isPlaying: false,
      volume: 100,
      isRepeating: false,
      currentTime: 0,
      duration: 0,
    })
    audio.updateTrackState('forest', { isPlaying: true, volume: 35 })
    audio.initTrack('forest', 'Replacement')
    expect(audio.tracks.forest).toMatchObject({ name: 'Forest', isPlaying: true, volume: 35 })
    expect(audio.availableTracks).toEqual([audio.tracks.forest])
    expect(audio.getPlayingTracks()).toEqual([audio.tracks.forest])
    audio.initTrack('silent', 'Silent')
    expect(audio.getPlayingTracks()).toHaveLength(1)
  })

  it('creates tracks for partial updates and removes them by file ID', () => {
    const audio = useAudioStore()
    audio.updateTrackState('new', { name: 'New', currentTime: 12 })
    expect(audio.tracks.new).toMatchObject({
      fileID: 'new',
      name: 'New',
      currentTime: 12,
      volume: 100,
    })
    audio.removeTrack('new')
    audio.removeTrack('missing')
    expect(audio.availableTracks).toEqual([])
  })

  it('ignores full sync until the listener enables audio', () => {
    const audio = useAudioStore()
    audio.initTrack('local', 'Local')
    audio.syncTracks([{ fileID: 'remote', isPlaying: true }])
    expect(Object.keys(audio.tracks)).toEqual(['local'])
  })

  it('reconciles an enabled session, preserving omitted fields and removing stale tracks', () => {
    const audio = useAudioStore()
    audio.enabled = true
    audio.initTrack('stale', 'Stale')
    audio.initTrack('keep', 'Keep')
    audio.updateTrackState('keep', { volume: 40 })
    audio.syncTracks([
      { fileID: 'keep', isPlaying: true },
      { fileID: 'new', name: 'New', volume: 60 },
      { name: 'No ID' },
    ])
    expect(Object.keys(audio.tracks).sort()).toEqual(['keep', 'new'])
    expect(audio.tracks.keep).toMatchObject({ name: 'Keep', volume: 40, isPlaying: true })
    expect(audio.tracks.new).toMatchObject({ name: 'New', volume: 60 })
    audio.syncTracks([])
    expect(audio.availableTracks).toEqual([])
  })

  it('tracks fades and preserves zero volume overrides', () => {
    const audio = useAudioStore()
    audio.setFading('forest', true)
    expect(audio.fadeStates.forest?.inProgress).toBe(true)
    audio.setFading('forest', false)
    expect(audio.fadeStates.forest?.inProgress).toBe(false)
    expect(audio.getTypeVolume('Music')).toBe(100)
    audio.setTypeVolume('Music', 0)
    expect(audio.getTypeVolume('Music')).toBe(0)
    audio.setTypeVolume('Ambiance', 55)
    expect(audio.getTypeVolume('Ambiance')).toBe(55)
  })

  it('keeps idle GM library entries when restoring server playback', () => {
    const audio = useAudioStore()
    audio.enabled = true
    audio.initTrack('idle', 'Idle')
    audio.updateTrackState('idle', { isPlaying: true, volume: 30 })
    audio.syncTracks([{ fileID: 'remote', name: 'Remote', isPlaying: true, currentTime: 12 }], true)
    expect(audio.tracks.idle).toMatchObject({ isPlaying: false, volume: 30, name: 'Idle' })
    expect(audio.tracks.remote).toMatchObject({ isPlaying: true, currentTime: 12 })
  })
})
