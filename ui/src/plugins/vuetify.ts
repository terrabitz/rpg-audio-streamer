import { mdiDelete, mdiHome, mdiMusic, mdiPause, mdiPlay, mdiVolumeHigh } from '@mdi/js'
import { createVuetify } from 'vuetify'
import { aliases, mdi } from 'vuetify/iconsets/mdi-svg'
import 'vuetify/styles'

export default createVuetify({
  icons: {
    defaultSet: 'mdi',
    aliases: {
      ...aliases,
      home: mdiHome,
      music: mdiMusic,
      play: mdiPlay,
      pause: mdiPause,
      delete: mdiDelete,
      volume: mdiVolumeHigh,
    },
    sets: {
      mdi,
    },
  },
})