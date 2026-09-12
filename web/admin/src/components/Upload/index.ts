import { withInstall } from '/@/utils';
import basicUpload from './src/BasicUpload.vue';
import defaultUpload from './src/DefaultUpload.vue';

export const BasicUpload = withInstall(basicUpload);
export const DefaultUpload = withInstall(defaultUpload);
