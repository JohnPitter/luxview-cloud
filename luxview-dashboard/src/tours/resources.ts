import type { Step } from 'react-joyride';

export const resourcesTourSteps: Step[] = [
  {
    target: 'body',
    placement: 'center',
    title: 'tour.resources.welcome.title',
    content: 'tour.resources.welcome.content',
    disableBeacon: true,
  },
  {
    target: '[data-tour="services-list"]',
    title: 'tour.resources.services.title',
    content: 'tour.resources.services.content',
  },
  {
    target: '[data-tour="add-service"]',
    title: 'tour.resources.addService.title',
    content: 'tour.resources.addService.content',
  },
];
