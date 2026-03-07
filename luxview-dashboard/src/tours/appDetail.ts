import type { Step } from 'react-joyride';

export const appDetailTourSteps: Step[] = [
  {
    target: 'body',
    placement: 'center',
    title: 'tour.appDetail.welcome.title',
    content: 'tour.appDetail.welcome.content',
    disableBeacon: true,
  },
  {
    target: '[data-tour="app-overview"]',
    title: 'tour.appDetail.overview.title',
    content: 'tour.appDetail.overview.content',
  },
  {
    target: '[data-tour="app-metrics"]',
    title: 'tour.appDetail.metrics.title',
    content: 'tour.appDetail.metrics.content',
  },
  {
    target: '[data-tour="app-logs"]',
    title: 'tour.appDetail.logs.title',
    content: 'tour.appDetail.logs.content',
  },
  {
    target: '[data-tour="app-services"]',
    title: 'tour.appDetail.services.title',
    content: 'tour.appDetail.services.content',
  },
  {
    target: '[data-tour="app-deploys"]',
    title: 'tour.appDetail.deploys.title',
    content: 'tour.appDetail.deploys.content',
  },
];
