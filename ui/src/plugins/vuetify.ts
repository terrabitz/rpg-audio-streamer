import IconLute from '@/components/icons/IconLute.vue';
import { mdiBug, mdiCircle, mdiContentCopy, mdiDelete, mdiHome, mdiMusic, mdiPause, mdiPlay, mdiRefresh, mdiRepeat, mdiVolumeHigh } from '@mdi/js';
import { h } from 'vue';
import { createVuetify, type IconProps, type IconSet } from 'vuetify';
import { aliases, mdi } from 'vuetify/iconsets/mdi-svg';
import 'vuetify/styles';

const customSvgNameToComponent: any = {
  lute: IconLute
};

const custom: IconSet = {
  component: (props: IconProps) => h(customSvgNameToComponent[props.icon as string]),
};

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
      copy: mdiContentCopy,
      bug: mdiBug,
      circle: mdiCircle,
      refresh: mdiRefresh,
    },
    sets: {
      mdi,
      custom,
    },
  },
})