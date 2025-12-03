import IconLute from '@/components/icons/IconLute.vue';
import { mdiAccountMusic, mdiBug, mdiCircle, mdiContentCopy, mdiContentSave, mdiDelete, mdiDotsVertical, mdiHeadphones, mdiHome, mdiLoading, mdiLogin, mdiMusic, mdiPause, mdiPlay, mdiRefresh, mdiRepeat, mdiRepeatOff, mdiUpload, mdiVolumeHigh, mdiVolumeLow, mdiVolumeMedium, mdiVolumeOff } from '@mdi/js';
import { h, type Component } from 'vue';
import { createVuetify, type IconProps, type IconSet } from 'vuetify';
import { aliases, mdi } from 'vuetify/iconsets/mdi-svg';
import 'vuetify/styles';

const customSvgNameToComponent: Record<string, Component> = {
  lute: IconLute
};

const custom: IconSet = {
  component: (props: IconProps) => {
    const component = customSvgNameToComponent[props.icon as string];
    if (!component) {
      return h(props.tag);
    }

    return h(props.tag, [h(component, { class: 'v-icon__svg' })]);
  },
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
      save: mdiContentSave
    },
    sets: {
      mdi,
      custom,
    },
  },
})