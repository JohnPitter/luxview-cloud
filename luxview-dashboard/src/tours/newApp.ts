import type { TourStep } from '../components/common/PageTour';

export const newAppTourSteps: TourStep[] = [
  {
    target: 'body',
    placement: 'center',
    title: 'tour.newApp.welcome.title',
    content: 'tour.newApp.welcome.content',
    disableBeacon: true,
  },
  {
    target: '[data-tour="repo-list"]',
    title: 'tour.newApp.repo.title',
    content: 'tour.newApp.repo.content',
  },
  {
    target: '[data-tour="deploy-config"]',
    title: 'tour.newApp.config.title',
    content: 'tour.newApp.config.content',
  },
];
