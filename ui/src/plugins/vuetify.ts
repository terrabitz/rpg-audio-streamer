import IconLute from '@/components/icons/IconLute.vue';
import { mdiAccountMusic, mdiBug, mdiCircle, mdiContentCopy, mdiDelete, mdiDotsVertical, mdiHeadphones, mdiHome, mdiLoading, mdiLogin, mdiMusic, mdiPause, mdiPlay, mdiRefresh, mdiRepeat, mdiRepeatOff, mdiTableLarge, mdiUpload, mdiVolumeHigh, mdiVolumeLow, mdiVolumeMedium, mdiVolumeOff } from '@mdi/js';
import { h } from 'vue';
import { createVuetify, type IconProps, type IconSet } from 'vuetify';
import { aliases, mdi } from 'vuetify/iconsets/mdi-svg';
import 'vuetify/styles';

const customSvgNameToComponent: any = {
  lute: IconLute
};

const custom: IconSet = {
  component: (props: IconProps) =>
    h(props.tag, [h(customSvgNameToComponent[props.icon as string], { class: 'v-icon__svg' })]),
}

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
      repeat: mdiRepeat,
      repeatOff: mdiRepeatOff,
      copy: mdiContentCopy,
      bug: mdiBug,
      circle: mdiCircle,
      refresh: mdiRefresh,
      loading: mdiLoading,
      upload: mdiUpload,
      volumeHigh: mdiVolumeHigh,
      volumeMedium: mdiVolumeMedium,
      volumeLow: mdiVolumeLow,
      volumeOff: mdiVolumeOff,
      dotsVertical: mdiDotsVertical,
      login: mdiLogin,
      accountMusic: mdiAccountMusic,
      headphones: mdiHeadphones,
    },
    sets: {
      mdi,
      custom,
    },
  },
})