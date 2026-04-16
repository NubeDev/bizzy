// marketing_plan_qa.js — conversational marketing plan builder.
//
// Two calling modes:
//
// 1. Form mode (REST):  _submit=false → all fields at once, _submit=true → validate+render
// 2. Chat mode (WS QA): _answers={...} → returns one question at a time, then prompt
//
// The same tool supports both the form UI and the conversational Q&A flow.

function handle(params) {
  // --- Chat mode: conversational Q&A via WebSocket ---
  if (params._answers !== undefined) {
    return chatMode(params._answers);
  }

  // --- Form mode: REST API ---
  if (!params._submit) {
    return formDefinition();
  }
  return formSubmit(params);
}

// ---- Chat mode: one question at a time ----

function chatMode(answers) {
  if (!answers.product) {
    return {
      type: "question",
      field: "product",
      label: "What product or feature is this marketing plan for?",
      input: "text",
      required: true,
      min_length: 2,
      placeholder: "e.g. Rubix Edge Controller, Nube Wireless Sensor"
    };
  }

  if (!answers.audience) {
    return {
      type: "question",
      field: "audience",
      label: "Who is the target audience for " + answers.product + "?",
      input: "text",
      required: false,
      default: "B2B IoT and building automation buyers",
      placeholder: "e.g. Facility managers, systems integrators"
    };
  }

  if (!answers.budget) {
    return {
      type: "question",
      field: "budget",
      label: "What's the budget range for this campaign?",
      input: "select",
      required: true,
      options: [
        { value: "< $10k", label: "Under $10k" },
        { value: "$10k - $50k", label: "$10k \u2013 $50k" },
        { value: "$50k - $100k", label: "$50k \u2013 $100k" },
        { value: "$100k+", label: "Over $100k" }
      ]
    };
  }

  if (!answers.timeline) {
    return {
      type: "question",
      field: "timeline",
      label: "What's the campaign timeline?",
      input: "select",
      required: true,
      options: [
        { value: "1 month", label: "1 month sprint" },
        { value: "1 quarter", label: "1 quarter" },
        { value: "6 months", label: "6 months" },
        { value: "1 year", label: "Full year" }
      ]
    };
  }

  if (!answers.channels) {
    return {
      type: "question",
      field: "channels",
      label: "Any specific channels you want to focus on?",
      input: "multi_select",
      required: false,
      options: [
        { value: "linkedin", label: "LinkedIn" },
        { value: "email", label: "Email campaigns" },
        { value: "events", label: "Trade shows & events" },
        { value: "content", label: "Blog & content" },
        { value: "paid", label: "Paid advertising" },
        { value: "partners", label: "Partner program" },
        { value: "webinars", label: "Webinars" },
        { value: "youtube", label: "YouTube / video" }
      ]
    };
  }

  // Conditional: if big budget, ask about agency support
  var budgetStr = "" + answers.budget;
  if ((budgetStr.indexOf("$50k") >= 0 || budgetStr.indexOf("$100k") >= 0) && !answers.agency) {
    return {
      type: "question",
      field: "agency",
      label: "With that budget, are you planning to use an agency or handle in-house?",
      input: "select",
      required: false,
      options: [
        { value: "in-house", label: "In-house team" },
        { value: "agency", label: "External agency" },
        { value: "hybrid", label: "Hybrid (both)" }
      ]
    };
  }

  if (!answers.notes) {
    return {
      type: "question",
      field: "notes",
      label: "Anything else we should know? (competitors, past campaigns, goals)",
      input: "textarea",
      required: false,
      max_length: 1000,
      placeholder: "Optional — press enter to skip"
    };
  }

  // All questions answered — render the prompt.
  return renderPrompt(answers);
}

// ---- Form mode: all fields at once (for REST/form UI) ----

function formDefinition() {
  return {
    type: "qa",
    title: "Marketing Plan Builder",
    description: "Answer a few questions to generate a targeted marketing plan.",
    fields: [
      {
        name: "product", label: "What product or feature is this plan for?",
        type: "text", required: true, min_length: 2,
        placeholder: "e.g. Rubix Edge Controller"
      },
      {
        name: "audience", label: "Target audience",
        type: "text", required: false,
        default: "B2B IoT and building automation buyers",
        placeholder: "e.g. Facility managers, systems integrators"
      },
      {
        name: "budget", label: "Budget range",
        type: "select", required: true,
        options: [
          { value: "< $10k", label: "Under $10k" },
          { value: "$10k - $50k", label: "$10k \u2013 $50k" },
          { value: "$50k - $100k", label: "$50k \u2013 $100k" },
          { value: "$100k+", label: "Over $100k" }
        ]
      },
      {
        name: "timeline", label: "Campaign timeline",
        type: "select", required: true,
        options: [
          { value: "1 month", label: "1 month" },
          { value: "1 quarter", label: "1 quarter" },
          { value: "6 months", label: "6 months" },
          { value: "1 year", label: "1 year" }
        ]
      },
      {
        name: "channels", label: "Focus channels",
        type: "multi_select", required: false,
        options: [
          { value: "linkedin", label: "LinkedIn" },
          { value: "email", label: "Email campaigns" },
          { value: "events", label: "Trade shows & events" },
          { value: "content", label: "Blog & content marketing" },
          { value: "paid", label: "Paid advertising" },
          { value: "partners", label: "Partner/integrator program" },
          { value: "webinars", label: "Webinars" },
          { value: "youtube", label: "YouTube / video" }
        ]
      },
      {
        name: "notes", label: "Anything else we should know?",
        type: "textarea", required: false, max_length: 1000,
        placeholder: "Competitor context, past campaigns, specific goals..."
      }
    ]
  };
}

function formSubmit(params) {
  var errors = [];
  if (!params.product || params.product.length < 2) {
    errors.push({ field: "product", message: "Product name is required (min 2 characters)" });
  }
  if (!params.budget) {
    errors.push({ field: "budget", message: "Please select a budget range" });
  }
  if (!params.timeline) {
    errors.push({ field: "timeline", message: "Please select a timeline" });
  }
  if (params.notes && params.notes.length > 1000) {
    errors.push({ field: "notes", message: "Notes must be under 1000 characters" });
  }
  if (errors.length > 0) {
    return { type: "validation_error", errors: errors };
  }

  return renderPrompt(params);
}

// ---- Shared: render the final prompt ----

function renderPrompt(answers) {
  var template = files.read("prompts/marketing-plan.md");
  var parts = template.split("---");
  var body = parts.slice(2).join("---").trim();

  body = body.replace(/\{\{product\}\}/g, answers.product || "");
  body = body.replace(/\{\{audience\}\}/g, answers.audience || "B2B IoT and building automation buyers");

  body += "\n\nBudget: " + (answers.budget || "not specified");
  body += "\nTimeline: " + (answers.timeline || "not specified");

  if (answers.channels) {
    body += "\nFocus channels: " + answers.channels;
  }
  if (answers.agency) {
    body += "\nExecution model: " + answers.agency;
  }
  if (answers.notes) {
    body += "\n\nAdditional context from the user:\n" + answers.notes;
  }

  return {
    type: "prompt",
    title: "Marketing Plan for " + answers.product,
    prompt: body
  };
}
